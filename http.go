package main

// MIT https://github.com/erred/gitreposerver

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"smart-git/middleware/loggin"
	"smart-git/middleware/timing"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/format/pktline"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	"github.com/gofiber/fiber/v2"
)

// RunHTTP 函数启动 HTTP 服务器，处理 Git 仓库的 upload-pack 服务。
//
// 参数:
//   - addr: 服务器监听地址，例如 ":8080"。
//   - baseRepoDir: Git 仓库的基础目录。
//
// 返回值:
//   - error: 如果服务器启动失败，则返回错误信息。
func RunHTTP(addr string, baseRepoDir string) error {
	log.Printf("Starting HTTP server on addr '%s'\n", addr)

	r := fiber.New()
	r.Use(loggin.Middleware()) // 添加请求日志记录中间件
	r.Use(timing.Middleware()) // 添加请求处理时间记录中间件

	r.Get("/:user/:repo/info/refs", httpInfoRefs(baseRepoDir))             // 处理仓库引用信息请求
	r.Post("/:user/:repo/git-upload-pack", httpGitUploadPack(baseRepoDir)) // 处理 git-upload-pack 请求

	// 404 路由处理
	r.Use(func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusNotFound)
	})

	err := r.Listen(addr)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("Error during ListenAndServe: %v\n", err)
		log.Printf("HTTP server failed to start on addr '%s'\n", addr)
		return err
	}
	log.Println("HTTP server stopped")
	return nil
}

// httpInfoRefs 函数处理 /info/refs 请求，用于服务发现和获取仓库引用信息。
//
// 该函数响应 Git 客户端的 info/refs 请求，用于客户端发现服务并获取仓库的引用信息（例如分支和标签）。
// 如果仓库不存在，则会尝试从 GitHub 克隆仓库。
//
// 参数:
//   - baseRepoDir: Git 仓库的基础目录。
//
// 返回值:
//   - fiber.Handler: Fiber 路由处理函数。
func httpInfoRefs(baseRepoDir string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		repoName := c.Params("repo")
		userName := c.Params("user")
		dir := baseRepoDir + "/" + userName + "/" + repoName

		_, err := os.Stat(dir)
		if errors.Is(err, os.ErrNotExist) {
			// 如果仓库目录不存在，则尝试克隆仓库 (CloneRepo 函数需自行实现)
			CloneRepo(dir, repoName, "https://github.com/"+userName+"/"+repoName)
		} else if err != nil {
			log.Printf("CloneRepo error: %v, repo: %s\n", err, repoName)
			return c.SendStatus(http.StatusInternalServerError)
		}

		c.Set("Content-Type", "application/x-git-upload-pack-advertisement")

		ep, err := transport.NewEndpoint("/")
		if err != nil {
			log.Printf("Error creating endpoint: %v, repo: %s\n", err, repoName)
			_, errResp := c.WriteString(err.Error())
			if errResp != nil {
				log.Printf("WriteString error: %v\n", errResp)
			}
			return c.SendStatus(http.StatusInternalServerError)
		}

		bfs := osfs.New(dir)
		ld := server.NewFilesystemLoader(bfs)
		svr := server.NewServer(ld)
		sess, err := svr.NewUploadPackSession(ep, nil)
		if err != nil {
			log.Printf("Error creating upload pack session: %v, repo: %s\n", err, repoName)
			_, errResp := c.WriteString(err.Error())
			if errResp != nil {
				log.Printf("WriteString error: %v\n", errResp)
			}
			return c.SendStatus(http.StatusInternalServerError)
		}

		ar, err := sess.AdvertisedReferencesContext(c.Context())
		if err != nil {
			_, errResp := c.WriteString(err.Error())
			if errResp != nil {
				log.Printf("WriteString error: %v\n", errResp)
			}
			log.Printf("Error getting advertised references: %v, repo: %s\n", err, repoName)
			return c.SendStatus(http.StatusInternalServerError)
		}

		ar.Prefix = [][]byte{
			[]byte("# service=git-upload-pack"),
			pktline.Flush,
		}
		err = ar.Encode(c)
		if err != nil {
			log.Printf("Error encoding advertised references: %v, repo: %s\n", err, repoName)
			_, errResp := c.WriteString(err.Error())
			if errResp != nil {
				log.Printf("WriteString error: %v\n", errResp)
			}
			return c.SendStatus(http.StatusInternalServerError)
		}
		return nil
	}
}

// httpGitUploadPack 函数处理 /git-upload-pack 请求，允许客户端推送代码到服务器。
//
// 该函数接收客户端发送的 pack 文件，并调用 Git 服务器的 upload-pack 会话来处理推送操作。
// 支持处理 gzip 压缩的请求体。
//
// 参数:
//   - baseRepoDir: Git 仓库的基础目录。
//
// 返回值:
//   - fiber.Handler: Fiber 路由处理函数。
func httpGitUploadPack(baseRepoDir string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		repoName := c.Params("repo")
		userName := c.Params("user")
		dir := baseRepoDir + "/" + userName + "/" + repoName

		c.Set("Content-Type", "application/x-git-upload-pack-result")

		bodyBytes := c.Request().Body()
		var bodyReader io.Reader = bytes.NewReader(bodyBytes)
		if c.Get("Content-Encoding") == "gzip" {
			gzipReader, err := gzip.NewReader(bytes.NewReader(bodyBytes))
			if err != nil {
				log.Printf("Error creating gzip reader: %v, repo: %s\n", err, repoName)
				_, errResp := c.WriteString(err.Error())
				if errResp != nil {
					log.Printf("WriteString error: %v\n", errResp)
				}
				return c.SendStatus(http.StatusInternalServerError)
			}
			defer gzipReader.Close()
			bodyReader = gzipReader
		}

		upr := packp.NewUploadPackRequest()
		err := upr.Decode(bodyReader)
		if err != nil {
			log.Printf("Error decoding upload pack request: %v, repo: %s\n", err, repoName)
			_, errResp := c.WriteString(err.Error())
			if errResp != nil {
				log.Printf("WriteString error: %v\n", errResp)
			}
			return c.SendStatus(http.StatusInternalServerError)
		}

		ep, err := transport.NewEndpoint("/")
		if err != nil {
			log.Printf("Error creating endpoint: %v, repo: %s\n", err, repoName)
			_, errResp := c.WriteString(err.Error())
			if errResp != nil {
				log.Printf("WriteString error: %v\n", errResp)
			}
			return c.SendStatus(http.StatusInternalServerError)
		}

		bfs := osfs.New(dir)
		ld := server.NewFilesystemLoader(bfs)
		svr := server.NewServer(ld)
		sess, err := svr.NewUploadPackSession(ep, nil)
		if err != nil {
			log.Printf("Error creating upload pack session: %v, repo: %s\n", err, repoName)
			_, errResp := c.WriteString(err.Error())
			if errResp != nil {
				log.Printf("WriteString error: %v\n", errResp)
			}
			return c.SendStatus(http.StatusInternalServerError)
		}

		res, err := sess.UploadPack(c.Context(), upr)
		if err != nil {
			_, errResp := c.WriteString(err.Error())
			if errResp != nil {
				log.Printf("WriteString error: %v\n", errResp)
			}
			log.Printf("Error during upload pack: %v, repo: %s\n", err, repoName)
			return c.SendStatus(http.StatusInternalServerError)
		}

		err = res.Encode(c)
		if err != nil {
			log.Printf("Error encoding upload pack result: %v, repo: %s\n", err, repoName)
			_, errResp := c.WriteString(err.Error())
			if errResp != nil {
				log.Printf("WriteString error: %v\n", errResp)
			}
			return c.SendStatus(http.StatusInternalServerError)
		}

		return nil
	}
}
