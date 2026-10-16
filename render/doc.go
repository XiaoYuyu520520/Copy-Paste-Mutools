package render

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Kind 描述当前文档的渲染类型。
type Kind string

const (
	// KindEmpty 尚未载入任何文档。
	KindEmpty Kind = "empty"
	// KindMarkdown 普通 Markdown/文本，走页内 marked 渲染。
	KindMarkdown Kind = "markdown"
	// KindMap 2D 激光 SLAM 地图，走页内 iframe 地图编辑器。
	KindMap Kind = "map"
)

// Document 是页面当前展示文档的快照，可直接 JSON 序列化给浏览器。
type Document struct {
	Kind    Kind   `json:"kind"`
	Name    string `json:"name"`
	Path    string `json:"path,omitempty"`
	Content string `json:"content,omitempty"`
	Version int    `json:"version"`
	Error   string `json:"error,omitempty"`
}

// markdownExts 视为纯文本、可直接渲染的扩展名。
var markdownExts = map[string]bool{
	".md": true, ".markdown": true, ".mdown": true,
	".txt": true, ".text": true, ".log": true,
}

// DocStore 保存「由原生窗口拖入的文档」，供渲染页拉取（/api/doc）与订阅（/api/events）。
//
// 页面里直接拖入的文件不经过这里：那是纯前端行为，无需落到服务端。
// 每次变更都会递增 Version 并推送给所有订阅者，页面据此热切换到新文档。
type DocStore struct {
	mu      sync.Mutex
	version int
	doc     Document
	mapHTML string // Kind == KindMap 时对应的地图页 HTML
	subs    map[chan Document]struct{}
}

// NewDocStore 建一个空文档源。
func NewDocStore() *DocStore {
	return &DocStore{doc: Document{Kind: KindEmpty}, subs: make(map[chan Document]struct{})}
}

// Current 返回当前文档快照。
func (s *DocStore) Current() Document {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doc
}

// MapHTML 返回当前地图页 HTML；当前不是地图时第二个返回值为 false。
func (s *DocStore) MapHTML() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mapHTML == "" {
		return "", false
	}
	return s.mapHTML, true
}

// Subscribe 订阅文档变更：返回接收通道与取消函数（取消可安全重复调用）。
// 通道容量为 1，只保留最新快照，订阅者不会阻塞发布方。
func (s *DocStore) Subscribe() (<-chan Document, func()) {
	ch := make(chan Document, 1)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()

	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, ok := s.subs[ch]; ok {
			delete(s.subs, ch)
			close(ch)
		}
	}
}

// LoadPaths 从拖入的路径中挑出可渲染文件并载入，返回新的文档快照。
// 一次拖入 .pgm + .yaml 会组合成地图文档；无论成功失败都会广播结果。
func (s *DocStore) LoadPaths(paths []string) (Document, error) {
	if len(paths) == 0 {
		err := fmt.Errorf("没有可用的本地文件")
		return s.fail(err), err
	}

	pgm, yamlPath, md := pickPaths(paths)
	switch {
	case pgm != "":
		doc, page, err := mapDocument(pgm, yamlPath)
		if err != nil {
			return s.fail(err), err
		}
		return s.publish(doc, page), nil
	case md != "":
		data, err := os.ReadFile(md)
		if err != nil {
			wrapped := fmt.Errorf("读取 %s 失败：%w", filepath.Base(md), err)
			return s.fail(wrapped), wrapped
		}
		doc := Document{
			Kind:    KindMarkdown,
			Name:    filepath.Base(md),
			Path:    md,
			Content: strings.TrimPrefix(string(data), "\ufeff"),
		}
		return s.publish(doc, ""), nil
	default:
		err := fmt.Errorf("不支持的文件类型：%s（支持 .md/.markdown/.txt/.pgm/.yaml）",
			filepath.Base(paths[0]))
		return s.fail(err), err
	}
}

// publish 更新当前文档并广播给所有订阅者（自动分配新版本号）。
func (s *DocStore) publish(doc Document, mapHTML string) Document {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.version++
	doc.Version = s.version
	s.doc, s.mapHTML = doc, mapHTML

	for ch := range s.subs {
		select {
		case ch <- doc:
		default:
			// 通道已满：丢掉旧的待发快照，保证订阅者最终拿到的是最新文档
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- doc:
			default:
			}
		}
	}
	return doc
}

// fail 广播一条只带错误信息的空文档，页面据此提示用户。
func (s *DocStore) fail(err error) Document {
	return s.publish(Document{Kind: KindEmpty, Error: err.Error()}, "")
}

// pickPaths 从拖入的多个路径里挑出一组可渲染文件：
// 优先 .pgm（自动配对同目录同名 .yaml），其次 Markdown/纯文本。
func pickPaths(paths []string) (pgm, yamlPath, md string) {
	for _, p := range paths {
		switch ext := strings.ToLower(filepath.Ext(p)); {
		case ext == ".pgm":
			if pgm == "" {
				pgm = p
			}
		case ext == ".yaml" || ext == ".yml":
			if yamlPath == "" {
				yamlPath = p
			}
		case markdownExts[ext]:
			if md == "" {
				md = p
			}
		}
	}

	// 只拖了 .yaml：用同目录同名 .pgm 兜底；找不到就当普通文本处理不了，直接报不支持
	if pgm == "" && yamlPath != "" {
		if cand := siblingWithExt(yamlPath, ".pgm"); fileExists(cand) {
			pgm = cand
		} else {
			yamlPath = ""
		}
	}
	// 只拖了 .pgm：自动找同目录同名 .yaml，找到就有比例尺/测量能力
	if pgm != "" && yamlPath == "" {
		if cand := siblingWithExt(pgm, ".yaml"); fileExists(cand) {
			yamlPath = cand
		} else if cand := siblingWithExt(pgm, ".yml"); fileExists(cand) {
			yamlPath = cand
		}
	}
	return pgm, yamlPath, md
}

// siblingWithExt 把路径换成同目录同主名的另一个扩展名。
func siblingWithExt(path, ext string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + ext
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// mapDocument 解析 .pgm(+.yaml) 并生成可直接给 iframe 的完整地图页。
func mapDocument(pgmPath, yamlPath string) (Document, string, error) {
	doc, err := PrepareMap(pgmPath, yamlPath)
	if err != nil {
		return Document{}, "", err
	}
	page, err := MapHTML(doc)
	if err != nil {
		return Document{}, "", err
	}
	name := filepath.Base(pgmPath)
	if yamlPath != "" {
		name += " + " + filepath.Base(yamlPath)
	}
	return Document{Kind: KindMap, Name: name, Path: pgmPath}, page, nil
}
