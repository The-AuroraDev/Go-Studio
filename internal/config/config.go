// config.go — 用户级与工作区级配置：默认值、深合并、原子读写。
// SPDX-License-Identifier: MIT

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// 主题取值。前端 theme 模块使用同一组字面量，两端必须保持一致。
const (
	ThemeDark   = "dark"
	ThemeLight  = "light"
	ThemeSystem = "system"
)

// 配置文件权限：配置未来可能承载敏感字段，因此只对所有者可读写。
const configFileMode fs.FileMode = 0o600

// Config 是前端与后端共享的全部持久化设置。字段使用 json tag 作为磁盘格式，
// 一旦发布即视为对外契约，改名需要配置迁移。
type Config struct {
	// Theme 决定界面配色：dark、light 或 system。
	Theme string `json:"theme"`
	// LogLevel 决定日志输出的最低级别：debug、info、warn 或 error。
	LogLevel string `json:"logLevel"`
}

// defaultJSON 是配置默认值，以 JSON 表达以便与磁盘文件走同一条合并路径。
const defaultJSON = `{"theme":"system","logLevel":"info"}`

// Default 返回内置默认配置，用于配置文件缺失或损坏时的回退。
func Default() Config {
	var cfg Config
	if err := json.Unmarshal([]byte(defaultJSON), &cfg); err != nil {
		// defaultJSON 是编译期常量，解析失败说明代码本身有误。
		panic(fmt.Sprintf("parse builtin default config: %v", err))
	}
	return cfg
}

// Load 按 默认值 < 用户级 < 工作区级 的顺序合并配置，返回最终生效的配置。
// 文件缺失不算错误；文件存在但内容非法则返回错误，交由调用方决定是否回退。
func Load(workspaceRoot string) (Config, error) {
	merged, err := decodeObject([]byte(defaultJSON))
	if err != nil {
		return Config{}, err
	}

	for _, path := range configLayers(workspaceRoot) {
		if path == "" {
			continue
		}
		layer, err := readObject(path)
		if err != nil {
			return Config{}, err
		}
		merged = mergeObjects(merged, layer)
	}

	var cfg Config
	if err := json.Unmarshal(mustMarshal(merged), &cfg); err != nil {
		return Config{}, fmt.Errorf("decode merged config: %w", err)
	}
	return cfg, nil
}

// Save 原子写入配置：先写同目录临时文件再 rename，避免写入中断产生半截文件。
func Save(path string, cfg Config) error {
	if path == "" {
		return errors.New("config path is empty")
	}
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), configFileName+".*")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Chmod(tmpName, configFileMode); err != nil {
		return fmt.Errorf("chmod temp config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace config %s: %w", path, err)
	}
	return nil
}

// SaveUser 把配置写回用户级配置文件。
func SaveUser(cfg Config) error {
	path, err := UserFile()
	if err != nil {
		return err
	}
	return Save(path, cfg)
}

// configLayers 返回参与合并的配置文件路径，工作区级在后因此覆盖更晚。
func configLayers(workspaceRoot string) []string {
	userFile, err := UserFile()
	if err != nil {
		// 用户配置目录不可用时退化为只合并工作区级，不阻断启动。
		return []string{WorkspaceFile(workspaceRoot)}
	}
	return []string{userFile, WorkspaceFile(workspaceRoot)}
}

// readObject 读取一个 JSON 对象文件。文件不存在时返回 nil 表示该层无贡献。
func readObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	return decodeObject(data)
}

// decodeObject 把一段 JSON 文本解析为对象，null 与非对象都视为空层。
func decodeObject(data []byte) (map[string]any, error) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("parse config json: %w", err)
	}
	return obj, nil
}

// mergeObjects 就地合并两个配置对象，overlay 中的值覆盖 base 中的值。
// 嵌套对象递归合并，这样工作区级只需写出要覆盖的字段。
func mergeObjects(base, overlay map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	for key, value := range overlay {
		if nested, ok := value.(map[string]any); ok {
			if existing, ok := base[key].(map[string]any); ok {
				base[key] = mergeObjects(existing, nested)
				continue
			}
		}
		base[key] = value
	}
	return base
}

// mustMarshal 只用于序列化刚由 json 解析得到的 map[string]any，
// 该类型必然可序列化，出错说明出现了程序缺陷。
func mustMarshal(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("marshal merged config: %v", err))
	}
	return data
}
