package main

// MIT https://github.com/erred/gitreposerver

import (
	"log"
	"net/http"
	"smart-git/database"

	"github.com/fenthope/compress"
	"github.com/fenthope/record"
	"github.com/infinite-iroha/touka"
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

	r := touka.Default()
	r.SetProtocols(&touka.ProtocolsConfig{
		Http1:           true,
		Http2_Cleartext: true,
	})

	// 添加中间件
	r.Use(record.Middleware())

	r.Use(compress.Compression(compress.DefaultCompressionConfig()))

	r.GET("/:user/:repo/info/refs", handleInfoRefs(baseRepoDir))    // 处理仓库引用信息请求
	r.POST("/:user/:repo/git-upload-pack", serviceRPC(baseRepoDir)) // 处理 git-upload-pack 请求

	r.GET("/healthz", func(c *touka.Context) {
		RenderWANF(c, http.StatusOK, &APIHealthResponse{
			Status:       "ok",
			RepoDir:      cfg.Server.BaseDir,
			DatabasePath: cfg.Database.Path,
			GithubBase:   "https://github.com",
		})
	})

	// info获取
	r.GET("/api/db/data", func(c *touka.Context) {
		allData, err := database.DB.GetAllData()
		if err != nil {
			RenderWANFError(c, http.StatusInternalServerError, err.Error())
			return
		}

		resp := make([]APIRepoRecord, 0, len(allData))
		for _, record := range allData {
			resp = append(resp, NewAPIRepoRecord(record))
		}
		RenderWANF(c, http.StatusOK, &APIRepoRecordList{Items: resp})
	})
	r.GET("/api/db/sum", func(c *touka.Context) {
		allData, err := database.DB.GetAllSumData()
		if err != nil {
			RenderWANFError(c, http.StatusInternalServerError, err.Error())
			return
		}

		resp := make([]APIRepoStats, 0, len(allData))
		for _, record := range allData {
			resp = append(resp, NewAPIRepoStats(record))
		}
		RenderWANF(c, http.StatusOK, &APIRepoStatsList{Items: resp})
	})

	// 404 路由处理
	r.NoRoute(func(c *touka.Context) {
		logInfo("404 Not Found, Path: %s", string(c.GetRequestURIPath())) // 使用 rc.Path() 获取路径
		c.Status(http.StatusNotFound)                                     // 发送 404 状态码
	})

	err := r.Run(
		touka.WithAddr(addr),
		touka.WithGracefulShutdownDefault(),
	)
	if err != nil {
		logError("Error starting HTTP server: %v\n", err)
		return err
	}
	log.Println("HTTP server stopped")
	return nil
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
/*
func httpGitUploadPack(c *touka.Context, baseRepoDir string) { // 使用 Hertz 的 Context 和 RequestContext

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
	ld := gitserver.NewFilesystemLoader(bfs, false)
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
}
*/
