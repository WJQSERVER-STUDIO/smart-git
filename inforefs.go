package main

import (
	"context"
	"fmt"
	"net/http"
	"smart-git/gitc"

	"github.com/go-git/go-billy/v6/osfs"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/transport"
	gitserver "github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/utils/ioutil"
	"github.com/infinite-iroha/touka"
)

func handleInfoRefs(baseRepoDir string) touka.HandlerFunc {
	return func(c *touka.Context) {
		w := c.Writer
		r := c.Request
		ctx := c.Context()

		repoName := c.Param("repo")
		userName := c.Param("user")
		serviceName := c.Query("service")
		if serviceName != "git-upload-pack" {
			logInfo("Full URI: %s", c.GetRequestURI())
			c.String(http.StatusForbidden, "Invalid service, Only Smart HTTP")
			logError("Invalid service, Only Smart HTTP")
			return
		}

		if err := ensureRepoReady(ctx, baseRepoDir, userName, repoName); err != nil {
			if err == plumbing.ErrReferenceNotFound {
				c.ErrorUseHandle(http.StatusNotFound, err)
				return
			}

			logError("ensure repo failed: %v\n", err)
			c.ErrorUseHandle(http.StatusInternalServerError, err)
			return
		}

		c.SetHeader("Content-Type", "application/x-git-upload-pack-advertisement")

		bfs := osfs.New(baseRepoDir)
		ld := gitserver.NewFilesystemLoader(bfs, true)
		epStr := "/" + userName + "/" + repoName
		c.Infof("epStr: %s\n", epStr)
		ep, err := transport.NewEndpoint(epStr)
		if err != nil {
			logError("Error creating endpoint: %v, repo: %s\n", err, repoName)
			c.ErrorUseHandle(http.StatusInternalServerError, err)
			return
		}

		st, err := ld.Load(ep)
		if err != nil {
			logError("Error loading repository: %v, repo: %s\n", err, repoName)
			c.Status(http.StatusNotFound)
			return
		}

		c.Header("Cache-Control", "no-cache, max-age=0, must-revalidate")
		service := transport.Service(serviceName)
		version := r.Header.Get("Git-Protocol")

		if service != "" {
			hdrNocache(w)
			w.Header().Set("Content-Type", fmt.Sprintf("application/x-git-%s-advertisement", service.Name()))

			switch service {
			case transport.UploadPackService:
				err = writeAdvertisedRefs(ctx, st, service, version, w)
			case transport.ReceivePackService:
				err = transport.ReceivePack(ctx, st, nil, ioutil.WriteNopCloser(w),
					&transport.ReceivePackOptions{
						GitProtocol:   version,
						AdvertiseRefs: true,
						StatelessRPC:  true,
					},
				)
			}
			if err != nil {
				logError("Error advertising refs: %v, repo: %s\n", err, repoName)
				return
			}
		} else {
			hdrNocache(w)
			sendFile(w, r, "text/plain; charset=utf-8")
		}
	}
}

func ensureRepoReady(ctx context.Context, baseRepoDir, userName, repoName string) error {
	if err := AddRequestCount(userName, repoName); err != nil {
		return err
	}

	gitURL := "https://github.com/" + userName + "/" + repoName
	return gitc.EnsureRepoReady(ctx, baseRepoDir, userName, repoName, gitURL, cfg)
}
