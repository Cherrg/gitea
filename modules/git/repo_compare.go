// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"container/list"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// CompareInfo represents needed information for comparing references.
type CompareInfo struct {
	MergeBase string
	Commits   *list.List
	NumFiles  int
}

// GetMergeBase checks and returns merge base of two branches.
func (repo *Repository) GetMergeBase(tmpRemote string, base, head string) (string, error) {
	if tmpRemote == "" {
		tmpRemote = "origin"
	}

	if tmpRemote != "origin" {
		tmpBaseName := "refs/remotes/" + tmpRemote + "/tmp_" + base
		// Fetch commit into a temporary branch in order to be able to handle commits and tags
		_, err := NewCommand("fetch", tmpRemote, base+":"+tmpBaseName).RunInDir(repo.Path)
		if err == nil {
			base = tmpBaseName
		}
	}

	stdout, err := NewCommand("merge-base", base, head).RunInDir(repo.Path)
	return strings.TrimSpace(stdout), err
}

// GetCompareInfo generates and returns compare information between base and head branches of repositories.
func (repo *Repository) GetCompareInfo(basePath, baseBranch, headBranch string) (_ *CompareInfo, err error) {
	var (
		remoteBranch string
		tmpRemote    string
	)

	// We don't need a temporary remote for same repository.
	if repo.Path != basePath {
		// Add a temporary remote
		tmpRemote = strconv.FormatInt(time.Now().UnixNano(), 10)
		if err = repo.AddRemote(tmpRemote, basePath, true); err != nil {
			return nil, fmt.Errorf("AddRemote: %v", err)
		}
		defer repo.RemoveRemote(tmpRemote)
	}

	compareInfo := new(CompareInfo)
	compareInfo.MergeBase, err = repo.GetMergeBase(tmpRemote, baseBranch, headBranch)
	if err == nil {
		// We have a common base
		logs, err := NewCommand("log", compareInfo.MergeBase+"..."+headBranch, prettyLogFormat).RunInDirBytes(repo.Path)
		if err != nil {
			return nil, err
		}
		compareInfo.Commits, err = repo.parsePrettyFormatLogToList(logs)
		if err != nil {
			return nil, fmt.Errorf("parsePrettyFormatLogToList: %v", err)
		}
	} else {
		compareInfo.Commits = list.New()
		compareInfo.MergeBase, err = GetFullCommitID(repo.Path, remoteBranch)
		if err != nil {
			compareInfo.MergeBase = remoteBranch
		}
	}

	// Count number of changed files.
	stdout, err := NewCommand("diff", "--name-only", remoteBranch+"..."+headBranch).RunInDir(repo.Path)
	if err != nil {
		return nil, err
	}
	compareInfo.NumFiles = len(strings.Split(stdout, "\n")) - 1

	return compareInfo, nil
}

// GetPatch generates and returns patch data between given revisions.
func (repo *Repository) GetPatch(base, head string) ([]byte, error) {
	return NewCommand("diff", "-p", "--binary", base, head).RunInDirBytes(repo.Path)
}

// GetFormatPatch generates and returns format-patch data between given revisions.
func (repo *Repository) GetFormatPatch(base, head string) (io.Reader, error) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	if err := NewCommand("format-patch", "--binary", "--stdout", base+"..."+head).
		RunInDirPipeline(repo.Path, stdout, stderr); err != nil {
		return nil, concatenateError(err, stderr.String())
	}
	return stdout, nil
}
