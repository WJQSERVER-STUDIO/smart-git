package main

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/protocol/packp"
	"github.com/go-git/go-git/v6/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v6/plumbing/storer"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/storage"
	"github.com/go-git/go-git/v6/utils/ioutil"
)

const defaultChunkSize = 4096

type flushResponseWriter struct {
	http.ResponseWriter
	log       *log.Logger
	chunkSize int
}

func (f *flushResponseWriter) ReadFrom(r io.Reader) (int64, error) {
	flusher := http.NewResponseController(f.ResponseWriter)

	var n int64
	p := make([]byte, f.chunkSize)
	for {
		nr, err := r.Read(p)
		if errors.Is(err, io.EOF) {
			break
		}
		nw, err := f.ResponseWriter.Write(p[:nr])
		if err != nil {
			logf(f.log, "error writing response: %v", err)
			renderStatusError(f.ResponseWriter, http.StatusInternalServerError)
			return n, err
		}
		if nr != nw {
			return n, err
		}
		n += int64(nr)
		if err := flusher.Flush(); err != nil {
			logf(f.log, "mismatched bytes written: expected %d, wrote %d", nr, nw)
			renderStatusError(f.ResponseWriter, http.StatusInternalServerError)
			return n, fmt.Errorf("%w: error while flush", err)
		}
	}

	return n, nil
}

func (f *flushResponseWriter) Close() error {
	return nil
}

type contextKey string

type service struct {
	pattern *regexp.Regexp
	method  string
	handler http.HandlerFunc
	svc     transport.Service
}

var services = []service{
	{regexp.MustCompile("(.*?)/HEAD$"), http.MethodGet, getTextFile, ""},
	{regexp.MustCompile("(.*?)/info/refs$"), http.MethodGet, getInfoRefs, ""},
	{regexp.MustCompile("(.*?)/objects/info/alternates$"), http.MethodGet, getTextFile, ""},
	{regexp.MustCompile("(.*?)/objects/info/http-alternates$"), http.MethodGet, getTextFile, ""},
	{regexp.MustCompile("(.*?)/objects/info/packs$"), http.MethodGet, getInfoPacks, ""},
	{regexp.MustCompile("(.*?)/objects/[0-9a-f]{2}/[0-9a-f]{38}$"), http.MethodGet, getLooseObject, ""},
	{regexp.MustCompile("(.*?)/objects/[0-9a-f]{2}/[0-9a-f]{62}$"), http.MethodGet, getLooseObject, ""},
	{regexp.MustCompile("(.*?)/objects/pack/pack-[0-9a-f]{40}\\.pack$"), http.MethodGet, getPackFile, ""},
	{regexp.MustCompile("(.*?)/objects/pack/pack-[0-9a-f]{64}\\.pack$"), http.MethodGet, getPackFile, ""},
	{regexp.MustCompile("(.*?)/objects/pack/pack-[0-9a-f]{40}\\.idx$"), http.MethodGet, getIdxFile, ""},
	{regexp.MustCompile("(.*?)/objects/pack/pack-[0-9a-f]{64}\\.idx$"), http.MethodGet, getIdxFile, ""},
	{regexp.MustCompile("(.*?)/git-upload-pack$"), http.MethodPost, serviceRpc, transport.UploadPackService},
	{regexp.MustCompile("(.*?)/git-receive-pack$"), http.MethodPost, serviceRpc, transport.ReceivePackService},
}

type Backend struct {
	Loader   transport.Loader
	ErrorLog *log.Logger
	Prefix   string
}

func NewBackend(loader transport.Loader) *Backend {
	if loader == nil {
		loader = transport.DefaultLoader
	}
	return &Backend{Loader: loader}
}

func (b *Backend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	urlPath := strings.TrimPrefix(r.URL.Path, b.Prefix)
	for _, s := range services {
		if m := s.pattern.FindStringSubmatch(urlPath); m != nil {
			if r.Method != s.method {
				renderStatusError(w, http.StatusMethodNotAllowed)
				return
			}

			repo := strings.TrimPrefix(m[1], "/")
			file := strings.Replace(urlPath, repo+"/", "", 1)
			ep, err := transport.NewEndpoint(repo)
			if err != nil {
				logf(b.ErrorLog, "error creating endpoint: %v", err)
				renderStatusError(w, http.StatusBadRequest)
				return
			}

			st, err := b.Loader.Load(ep)
			if err != nil {
				logf(b.ErrorLog, "error loading repository: %v", err)
				renderStatusError(w, http.StatusNotFound)
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, contextKey("errorLog"), b.ErrorLog)
			ctx = context.WithValue(ctx, contextKey("repo"), m[1])
			ctx = context.WithValue(ctx, contextKey("file"), file)
			ctx = context.WithValue(ctx, contextKey("service"), s.svc)
			ctx = context.WithValue(ctx, contextKey("storer"), st)
			ctx = context.WithValue(ctx, contextKey("endpoint"), ep)

			s.handler(w, r.WithContext(ctx))
			return
		}
	}

	renderStatusError(w, http.StatusNotFound)
}

func logf(logger *log.Logger, format string, v ...interface{}) {
	if logger != nil {
		logger.Printf(format, v...)
	}
}

func serviceRpc(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st, ok := ctx.Value(contextKey("storer")).(storage.Storer)
	if !ok {
		renderStatusError(w, http.StatusInternalServerError)
		return
	}
	svc, ok := ctx.Value(contextKey("service")).(transport.Service)
	if !ok {
		renderStatusError(w, http.StatusInternalServerError)
		return
	}
	errorLog, ok := ctx.Value(contextKey("errorLog")).(*log.Logger)
	if !ok {
		renderStatusError(w, http.StatusInternalServerError)
		return
	}
	version := r.Header.Get("Git-Protocol")
	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))

	expectedContentType := strings.ToLower(fmt.Sprintf("application/x-git-%s-request", svc.Name()))
	if contentType != expectedContentType {
		renderStatusError(w, http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", fmt.Sprintf("application/x-git-%s-result", svc.Name()))
	w.Header().Set("Connection", "Keep-Alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	var reader io.ReadCloser
	var err error
	switch r.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(r.Body)
		if err != nil {
			logf(errorLog, "error creating gzip reader: %v", err)
			renderStatusError(w, http.StatusInternalServerError)
			return
		}
		defer reader.Close() //nolint:errcheck
	default:
		reader = r.Body
	}

	frw := &flushResponseWriter{ResponseWriter: w, log: errorLog, chunkSize: defaultChunkSize}

	switch svc {
	case transport.UploadPackService:
		err = transport.UploadPack(ctx, st, reader, frw,
			&transport.UploadPackOptions{
				GitProtocol:   version,
				AdvertiseRefs: false,
				StatelessRPC:  true,
			})
	case transport.ReceivePackService:
		err = transport.ReceivePack(ctx, st, reader, frw,
			&transport.ReceivePackOptions{
				GitProtocol:   version,
				AdvertiseRefs: false,
				StatelessRPC:  true,
			})
	default:
		logf(errorLog, "unknown service: %s", svc.Name())
		renderStatusError(w, http.StatusNotFound)
		return
	}
	if err != nil {
		logf(errorLog, "error processing request: %v", err)
		renderStatusError(w, http.StatusInternalServerError)
		return
	}
}

func sendFile(w http.ResponseWriter, r *http.Request, contentType string) {
	ctx := r.Context()
	st, ok := ctx.Value(contextKey("storer")).(storage.Storer)
	if !ok {
		renderStatusError(w, http.StatusInternalServerError)
		return
	}
	fss, ok := st.(storer.FilesystemStorer)
	if !ok {
		renderStatusError(w, http.StatusNotFound)
		return
	}
	errorLog, ok := ctx.Value(contextKey("errorLog")).(*log.Logger)
	if !ok {
		renderStatusError(w, http.StatusInternalServerError)
		return
	}

	file, ok := ctx.Value(contextKey("file")).(string)
	if !ok {
		renderStatusError(w, http.StatusInternalServerError)
		return
	}
	fs := fss.Filesystem()
	f, err := fs.Open(file)
	if err != nil {
		renderStatusError(w, http.StatusNotFound)
		return
	}
	defer f.Close() //nolint:errcheck

	stat, err := fs.Lstat(file)
	if err != nil || !stat.Mode().IsRegular() {
		renderStatusError(w, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
	w.Header().Set("Last-Modified", stat.ModTime().Format(http.TimeFormat))

	frw := &flushResponseWriter{ResponseWriter: w, log: errorLog, chunkSize: defaultChunkSize}
	if _, err := io.Copy(frw, f); err != nil {
		logf(errorLog, "error writing response: %v", err)
		renderStatusError(w, http.StatusInternalServerError)
		return
	}
}

func getTextFile(w http.ResponseWriter, r *http.Request) {
	hdrNocache(w)
	sendFile(w, r, "text/plain; charset=utf-8")
}

func getInfoRefs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st, ok := ctx.Value(contextKey("storer")).(storage.Storer)
	if !ok {
		renderStatusError(w, http.StatusInternalServerError)
		return
	}
	errorLog, ok := ctx.Value(contextKey("errorLog")).(*log.Logger)
	if !ok {
		renderStatusError(w, http.StatusInternalServerError)
		return
	}

	service := transport.Service(r.URL.Query().Get("service"))
	version := r.Header.Get("Git-Protocol")

	if service != "" {
		hdrNocache(w)
		w.Header().Set("Content-Type", fmt.Sprintf("application/x-git-%s-advertisement", service.Name()))

		var err error
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
			logf(errorLog, "error processing request: %v", err)
			renderStatusError(w, http.StatusInternalServerError)
			return
		}
	} else {
		hdrNocache(w)
		sendFile(w, r, "text/plain; charset=utf-8")
	}
}

func getInfoPacks(w http.ResponseWriter, r *http.Request) {
	hdrCacheForever(w)
	sendFile(w, r, "text/plain; charset=utf-8")
}

func getLooseObject(w http.ResponseWriter, r *http.Request) {
	hdrCacheForever(w)
	sendFile(w, r, "application/x-git-loose-object")
}

func getPackFile(w http.ResponseWriter, r *http.Request) {
	hdrCacheForever(w)
	sendFile(w, r, "application/x-git-packed-objects")
}

func getIdxFile(w http.ResponseWriter, r *http.Request) {
	hdrCacheForever(w)
	sendFile(w, r, "application/x-git-packed-objects-toc")
}

func renderStatusError(w http.ResponseWriter, code int) {
	http.Error(w, fmt.Sprintf("%d %s", code, http.StatusText(code)), code)
}

func hdrNocache(w http.ResponseWriter) {
	w.Header().Set("Expires", "Fri, 01 Jan 1980 00:00:00 GMT")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Cache-Control", "no-cache, max-age=0, must-revalidate")
}

func hdrCacheForever(w http.ResponseWriter) {
	now := time.Now()
	expires := now.Add(365 * 24 * time.Hour)
	w.Header().Set("Date", now.Format(http.TimeFormat))
	w.Header().Set("Expires", expires.Format(http.TimeFormat))
	w.Header().Set("Cache-Control", "public, max-age=31536000")
}

func writeAdvertisedRefs(
	ctx context.Context,
	st storage.Storer,
	service transport.Service,
	version string,
	w io.Writer,
) error {
	_ = ctx
	_ = version

	if service != transport.UploadPackService && service != transport.ReceivePackService {
		return fmt.Errorf("unsupported service: %s", service.Name())
	}

	ar := packp.NewAdvRefs()
	ar.Capabilities.Set(capability.Agent, capability.DefaultAgent()) //nolint:errcheck
	ar.Capabilities.Set(capability.OFSDelta)                         //nolint:errcheck
	ar.Capabilities.Set(capability.Sideband64k)                      //nolint:errcheck

	if service == transport.ReceivePackService {
		ar.Capabilities.Set(capability.NoThin)       //nolint:errcheck
		ar.Capabilities.Set(capability.DeleteRefs)   //nolint:errcheck
		ar.Capabilities.Set(capability.ReportStatus) //nolint:errcheck
		ar.Capabilities.Set(capability.PushOptions)  //nolint:errcheck
		ar.Capabilities.Set(capability.Quiet)        //nolint:errcheck
	} else {
		ar.Capabilities.Set(capability.MultiACK)         //nolint:errcheck
		ar.Capabilities.Set(capability.MultiACKDetailed) //nolint:errcheck
		ar.Capabilities.Set(capability.Sideband)         //nolint:errcheck
		ar.Capabilities.Set(capability.SymRef)           //nolint:errcheck
		ar.Capabilities.Set(capability.Shallow)          //nolint:errcheck
	}

	if err := addAdvertisedReferences(st, ar, service == transport.UploadPackService); err != nil {
		return err
	}

	if err := (&packp.SmartReply{Service: service.String()}).Encode(w); err != nil {
		return err
	}

	return ar.Encode(w)
}

func addAdvertisedReferences(st storage.Storer, ar *packp.AdvRefs, addHead bool) error {
	iter, err := st.IterReferences()
	if err != nil {
		return err
	}

	return iter.ForEach(func(r *plumbing.Reference) error {
		hash, name := r.Hash(), r.Name()
		switch r.Type() {
		case plumbing.SymbolicReference:
			ref, err := storer.ResolveReference(st, r.Target())
			if errors.Is(err, plumbing.ErrReferenceNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			hash = ref.Hash()
		}

		if name == plumbing.HEAD {
			if !addHead {
				return nil
			}
			ar.Capabilities.Add(capability.SymRef, fmt.Sprintf("%s:%s", name, r.Target())) //nolint:errcheck
			ar.Head = &hash
		}

		ar.References[name.String()] = hash
		if r.Name().IsTag() {
			if tag, err := object.GetTag(st, hash); err == nil {
				ar.Peeled[name.String()] = tag.Target
			}
		}

		return nil
	})
}
