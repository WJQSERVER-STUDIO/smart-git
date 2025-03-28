package main

// MIT https://github.com/erred/gitreposerver

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"smart-git/database"
	"smart-git/gitc"
	"smart-git/middleware/loggin"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/pktline"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gitserver "github.com/go-git/go-git/v5/plumbing/transport/server"

	//hresp "github.com/cloudwego/hertz/pkg/protocol/http1/resp"
	rgzip "github.com/hertz-contrib/gzip"
	"github.com/hertz-contrib/http2/factory"
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
	logInfo("Starting HTTP server on addr '%s'\n", addr)

	r := server.New(
		server.WithHostPorts(addr),
		server.WithH2C(true),
	)

	r.AddProtocol("h2", factory.NewServerFactory())

	// 添加中间件
	r.Use(rgzip.Gzip(rgzip.DefaultCompression))

	r.Use(loggin.Middleware()) //  适配 loggin 中间件

	r.GET("/:user/:repo/info/refs", func(ctx context.Context, c *app.RequestContext) {
		httpInfoRefs(ctx, c, baseRepoDir)
	}) // 处理仓库引用信息请求
	r.POST("/:user/:repo/git-upload-pack", func(ctx context.Context, c *app.RequestContext) {
		httpGitUploadPack(ctx, c, baseRepoDir)
	}) // 处理 git-upload-pack 请求

	// info获取
	r.GET("/api/db/data", func(ctx context.Context, c *app.RequestContext) {
		allData, err := database.DB.GetAllData()
		if err != nil {
			c.Error(err)                             // 使用 Hertz 的 Error Handling
			c.Status(http.StatusInternalServerError) // 发送 500 状态码
			return
		}
		c.JSON(http.StatusOK, allData) // 使用 Hertz 的 JSON 响应
	})
	r.GET("/api/db/sum", func(ctx context.Context, c *app.RequestContext) {
		allData, err := database.DB.GetAllSumData()
		if err != nil {
			c.Error(err)                             // 使用 Hertz 的 Error Handling
			c.Status(http.StatusInternalServerError) // 发送 500 状态码
			return
		}
		c.JSON(http.StatusOK, allData) // 使用 Hertz 的 JSON 响应
	})

	// 404 路由处理
	r.NoRoute(func(ctx context.Context, c *app.RequestContext) {
		logInfo("404 Not Found, Path: %s", string(c.Path())) // 使用 rc.Path() 获取路径
		c.Status(http.StatusNotFound)                        // 发送 404 状态码
	})

	r.Spin()
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
func httpInfoRefs(ctx context.Context, c *app.RequestContext, baseRepoDir string) {

	repoName := c.Param("repo") // 使用 rc.Param 获取路由参数
	userName := c.Param("user") // 使用 rc.Param 获取路由参数

	dir := baseRepoDir + "/" + userName + "/" + repoName

	// 增加统计次数
	err := AddRequestCount(userName, repoName)
	if err != nil {
		logError("增加请求次数失败: %v\n", err)
		c.Error(err) // 使用 Hertz 的 Error Handling
		c.Status(http.StatusInternalServerError)
		return
	}

	gitUrl := "https://github.com/" + userName + "/" + repoName

	err = gitc.CloneRepo(dir, userName, repoName, gitUrl, cfg)
	if err != nil && err != plumbing.ErrReferenceNotFound {
		logError("CloneRepo error: %v\n", err)
		c.Error(err) // 使用 Hertz 的 Error Handling
		return
	} else if err == plumbing.ErrReferenceNotFound {
		logError("Repo not found: %v\n", err)
		c.Status(http.StatusNotFound) // 发送 404 状态码
		return
	}

	if c.Query("service") != "git-upload-pack" {
		logInfo("Full URI: %s", c.Request.URI().String())
		c.String(http.StatusForbidden, "Invalid service, Only Smart HTTP")
		logError("Invalid service, Only Smart HTTP")
		return
	}

	c.SetContentType("application/x-git-upload-pack-advertisement") // 使用 rc.SetContentType 设置 Content-Type

	ep, err := transport.NewEndpoint("/")
	if err != nil {
		logError("Error creating endpoint: %v, repo: %s\n", err, repoName)
		c.Error(err) // 使用 Hertz 的 Error Handling
		_, errResp := c.WriteString(err.Error())
		if errResp != nil {
			logError("WriteString error: %v\n", errResp)
		}
		c.Status(http.StatusInternalServerError)
		return
	}

	bfs := osfs.New(dir)
	ld := gitserver.NewFilesystemLoader(bfs)
	svr := gitserver.NewServer(ld)
	sess, err := svr.NewUploadPackSession(ep, nil)
	if err != nil {
		logError("Error creating upload pack session: %v, repo: %s\n", err, repoName)
		c.Error(err) // 使用 Hertz 的 Error Handling
		_, errResp := c.WriteString(err.Error())
		if errResp != nil {
			logError("WriteString error: %v\n", errResp)
		}
		c.Status(http.StatusInternalServerError)
		return
	}

	ar, err := sess.AdvertisedReferencesContext(ctx) // 使用 context.Context
	if err != nil {
		_, errResp := c.WriteString(err.Error())
		if errResp != nil {
			logError("WriteString error: %v\n", errResp)
		}
		logError("Error getting advertised references: %v, repo: %s\n", err, repoName)
		c.Status(http.StatusInternalServerError)
		return
	}

	//c.Response.HijackWriter(hresp.NewChunkedBodyWriter(&c.Response, c.GetWriter()))

	ar.Prefix = [][]byte{
		[]byte("# service=git-upload-pack"),
		pktline.Flush,
	}
	//writer := c.Response.BodyWriter()

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		err = ar.Encode(pw)
		if err != nil {
			logError("Error encoding advertised references: %v, repo: %s\n", err, repoName)
			_, errResp := c.WriteString(err.Error())
			if errResp != nil {
				logError("WriteString error: %v\n", errResp)
			}
			c.Status(http.StatusInternalServerError)
			return
		}
	}()

	c.SetBodyStream(pr, -1)

	/*

		err = ar.Encode(writer)
		if err != nil {
			logError("Error encoding advertised references: %v, repo: %s\n", err, repoName)
			_, errResp := c.WriteString(err.Error())
			if errResp != nil {
				logError("WriteString error: %v\n", errResp)
			}
			c.Status(http.StatusInternalServerError)
			return
		}
	*/
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
//   - app.Handler: Hertz 路由处理函数。
func httpGitUploadPack(ctx context.Context, c *app.RequestContext, baseRepoDir string) { // 使用 Hertz 的 Context 和 RequestContext

	repoName := c.Param("repo") // 使用 rc.Param 获取路由参数
	if repoName == "" {
		logError("repoName is empty")
		c.Error(errors.New("repoName is empty")) // 使用 Hertz 的 Error Handling
		c.Status(http.StatusInternalServerError)
		return
	}
	userName := c.Param("user")
	dir := baseRepoDir + "/" + userName + "/" + repoName

	c.SetContentType("application/x-git-upload-pack-result") // 使用 rc.SetContentType 设置 Content-Type

	bodyBytes := c.Request.Body() // 使用 rc.Request.Body() 获取请求体
	var bodyReader io.Reader = bytes.NewReader(bodyBytes)

	if string(c.GetHeader("Content-Encoding")) == "gzip" {
		gzipReader, err := gzip.NewReader(bytes.NewReader(bodyBytes))
		if err != nil {
			logError("Error creating gzip reader: %v, repo: %s\n", err, repoName)
			c.Error(err)
			_, errResp := c.WriteString(err.Error())
			if errResp != nil {
				logError("WriteString error: %v\n", errResp)
			}
			c.Status(http.StatusInternalServerError)
			return
		}
		defer gzipReader.Close()
		bodyReader = gzipReader
	}

	upr := packp.NewUploadPackRequest()
	err := upr.Decode(bodyReader)
	if err != nil {
		// 尝试读取并记录请求体的前几行，以便诊断问题
		bodyBuffer := new(bytes.Buffer)
		tee := io.TeeReader(bytes.NewReader(bodyBytes), bodyBuffer)
		firstLine := make([]byte, 200) // 读取前 200 字节
		n, _ := tee.Read(firstLine)
		logError("First part of upload pack request body: %s\n", string(firstLine[:n]))
		logError("Error decoding upload pack request: %v, repo: %s\n", err, repoName)
		c.Error(err) // 使用 Hertz 的 Error Handling
		_, errResp := c.WriteString(err.Error())
		if errResp != nil {
			logError("WriteString error: %v\n", errResp)
		}
		c.Status(http.StatusInternalServerError)
		return
	}

	ep, err := transport.NewEndpoint("/")
	if err != nil {
		logError("Error creating endpoint: %v, repo: %s\n", err, repoName)
		c.Error(err) // 使用 Hertz 的 Error Handling
		_, errResp := c.WriteString(err.Error())
		if errResp != nil {
			logError("WriteString error: %v\n", errResp)
		}
		c.Status(http.StatusInternalServerError)
		return
	}

	bfs := osfs.New(dir)
	ld := gitserver.NewFilesystemLoader(bfs)
	svr := gitserver.NewServer(ld)
	sess, err := svr.NewUploadPackSession(ep, nil)
	if err != nil {
		logError("Error creating upload pack session: %v, repo: %s\n", err, repoName)
		c.Error(err) // 使用 Hertz 的 Error Handling
		_, errResp := c.WriteString(err.Error())
		if errResp != nil {
			logError("WriteString error: %v\n", errResp)
		}
		c.Status(http.StatusInternalServerError)
		return
	}

	res, err := sess.UploadPack(ctx, upr) // 使用 context.Context
	if err != nil {
		_, errResp := c.WriteString(err.Error())
		if errResp != nil {
			logError("WriteString error: %v\n", errResp)
		}
		logError("Error during upload pack: %v, repo: %s\n", err, repoName)
		c.Status(http.StatusInternalServerError)
		return
	}

	//c.Response.HijackWriter(hresp.NewChunkedBodyWriter(&c.Response, c.GetWriter()))

	//writer := c.Response.BodyWriter()

	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()
		err = res.Encode(pw)
		if err != nil {
			logError("Error encoding upload pack result: %v, repo: %s\n", err, repoName)
			_, errResp := c.WriteString(err.Error())
			if errResp != nil {
				logError("WriteString error: %v\n", errResp)
			}
			c.Status(http.StatusInternalServerError)
			return
		}

	}()

	c.SetBodyStream(pr, -1)

	/*
	   err = res.Encode(writer) // 使用 c.Response.BodyWriter() 作为 io.Writer

	   	if err != nil {
	   		logError("Error encoding upload pack result: %v, repo: %s\n", err, repoName)
	   		_, errResp := c.WriteString(err.Error())
	   		if errResp != nil {
	   			logError("WriteString error: %v\n", errResp)
	   		}
	   		c.Status(http.StatusInternalServerError)
	   		return
	   	}
	*/
}
