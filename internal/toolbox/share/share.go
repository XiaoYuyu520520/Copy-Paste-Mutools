// Package share 实现局域网文件共享服务。
//
// 迁移自 legacy/python/.../lan-share-server/backend/server_for_packaging.py
// （FastAPI + uvicorn + PyInstaller 打包）。这里改成 Go 原生 net/http：
//   - GET  /api/files[/{path}]  列出目录
//   - POST /api/download/batch  多选打包成 zip 下载
//   - GET  /                    前端页面
//
// 相比 Python 版本的优势：单文件、免 Python 运行时、启动快、内存占用小。
package share

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Entry 是目录列表中的一个条目。
type Entry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	IsDirectory bool   `json:"is_directory"`
	Size        int64  `json:"size,omitempty"`
	ModTime     int64  `json:"mod_time,omitempty"`
}

// Options 是共享服务的配置。
type Options struct {
	// SharedDir 是对外共享的根目录。
	SharedDir string
	// Addr 是监听地址，如 "0.0.0.0:8000"。
	Addr string
	// FrontendFS 是前端静态资源；为空则只提供 API。
	FrontendFS http.FileSystem
}

// Server 是共享服务。
type Server struct {
	opts   Options
	root   string
	mux    *http.ServeMux
	server *http.Server
}

// New 创建一个共享服务实例。
func New(opts Options) (*Server, error) {
	root, err := filepath.Abs(opts.SharedDir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("共享目录不可用: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("共享路径不是目录: %s", root)
	}
	if opts.Addr == "" {
		opts.Addr = "0.0.0.0:8000"
	}

	s := &Server{opts: opts, root: root, mux: http.NewServeMux()}
	s.routes()
	s.server = &http.Server{Addr: opts.Addr, Handler: s.mux}
	return s, nil
}

// routes 注册 HTTP 路由。
func (s *Server) routes() {
	s.mux.HandleFunc("/api/files", s.handleList)
	s.mux.HandleFunc("/api/files/", s.handleList)
	s.mux.HandleFunc("/api/download/batch", s.handleBatchDownload)
	s.mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "root": s.root})
	})
	if s.opts.FrontendFS != nil {
		s.mux.Handle("/", http.FileServer(s.opts.FrontendFS))
	}
}

// ListenAndServe 启动服务并阻塞。
func (s *Server) ListenAndServe() error {
	return s.server.ListenAndServe()
}

// Shutdown 关闭服务。
func (s *Server) Shutdown() error {
	return s.server.Close()
}

// handleList 列出目录内容。
func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	sub := strings.TrimPrefix(r.URL.Path, "/api/files")
	sub = strings.Trim(sub, "/")

	dir, err := s.safeResolve(sub)
	if err != nil {
		http.Error(w, "拒绝访问: "+err.Error(), http.StatusForbidden)
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		http.Error(w, "读取目录失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	list := make([]Entry, 0, len(entries))
	for _, e := range entries {
		full := filepath.Join(dir, e.Name())
		rel, _ := filepath.Rel(s.root, full)
		item := Entry{
			Name:        e.Name(),
			Path:        filepath.ToSlash(rel),
			IsDirectory: e.IsDir(),
		}
		if info, err := e.Info(); err == nil {
			item.Size = info.Size()
			item.ModTime = info.ModTime().Unix()
		}
		list = append(list, item)
	}
	// 目录在前，同层按名称不区分大小写排序（与原 Python 版一致）。
	sort.Slice(list, func(i, j int) bool {
		if list[i].IsDirectory != list[j].IsDirectory {
			return list[i].IsDirectory
		}
		return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
	})

	writeJSON(w, http.StatusOK, list)
}

// batchRequest 是批量下载的请求体。
type batchRequest struct {
	Paths []string `json:"paths"`
}

// handleBatchDownload 把选中的文件/目录打包成 zip。
func (s *Server) handleBatchDownload(w http.ResponseWriter, r *http.Request) {
	var req batchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求体解析失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Paths) == 0 {
		http.Error(w, "paths 不能为空", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="shared_files.zip"`)

	zw := zip.NewWriter(w)
	defer zw.Close()

	for _, p := range req.Paths {
		full, err := s.safeResolve(p)
		if err != nil {
			continue
		}
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		if info.IsDir() {
			if err := addDirToZip(zw, full, s.root); err != nil {
				return
			}
			continue
		}
		if err := addFileToZip(zw, full, s.root); err != nil {
			return
		}
	}
}

// safeResolve 把相对路径解析成绝对路径，并确保结果仍在共享根目录内。
//
// 这是防止 ".." 越权访问的关键：解析后必须仍在 root 之下。
func (s *Server) safeResolve(rel string) (string, error) {
	rel = filepath.FromSlash(strings.TrimSpace(rel))
	if rel == "" || rel == "." {
		return s.root, nil
	}
	full := filepath.Clean(filepath.Join(s.root, rel))
	if !isInside(s.root, full) {
		return "", fmt.Errorf("路径越界")
	}
	return full, nil
}

// isInside 判断 child 是否位于 parent 之内（含相等）。
func isInside(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !strings.HasPrefix(rel, "..")
}

// addFileToZip 把单个文件加入压缩包。
func addFileToZip(zw *zip.Writer, full, root string) error {
	rel, err := filepath.Rel(root, full)
	if err != nil {
		return err
	}
	w, err := zw.Create(filepath.ToSlash(rel))
	if err != nil {
		return err
	}
	src, err := os.Open(full)
	if err != nil {
		return err
	}
	defer src.Close()
	_, err = io.Copy(w, src)
	return err
}

// addDirToZip 递归把目录加入压缩包。
func addDirToZip(zw *zip.Writer, dir, root string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		return addFileToZip(zw, path, root)
	})
}

// LocalAddrs 返回本机可用于局域网访问的 IPv4 地址列表。
//
// addr 用于解析出端口号，方便直接拼出可点击的访问链接。
func LocalAddrs(addr string) []string {
	out := []string{}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return out
	}
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() {
			continue
		}
		if ip4 := ipNet.IP.To4(); ip4 != nil {
			out = append(out, ip4.String())
		}
	}
	return out
}

// writeJSON 写出 JSON 响应。
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
