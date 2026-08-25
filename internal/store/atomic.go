package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func (s *JSONStore) persistLocked() (bool, error) {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return false, fmt.Errorf("创建数据目录失败：%w", err)
	}
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return false, fmt.Errorf("编码快照失败：%w", err)
	}
	tmp, err := os.CreateTemp(dir, ".snapshot-*.tmp")
	if err != nil {
		return false, fmt.Errorf("创建临时快照失败：%w", err)
	}
	tmpName := tmp.Name()
	clean := func() { _ = tmp.Close(); _ = os.Remove(tmpName) }
	if err := tmp.Chmod(0o640); err != nil {
		clean()
		return false, err
	}
	if _, err := tmp.Write(b); err != nil {
		clean()
		return false, fmt.Errorf("写入临时快照失败：%w", err)
	}
	if err := tmp.Sync(); err != nil {
		clean()
		return false, fmt.Errorf("同步临时快照失败：%w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return false, err
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		_ = os.Remove(tmpName)
		return false, fmt.Errorf("替换快照失败：%w", err)
	}
	d, err := os.Open(dir)
	if err != nil {
		return true, fmt.Errorf("快照已替换但打开数据目录失败：%w", err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return true, fmt.Errorf("快照已替换但同步数据目录失败：%w", err)
	}
	return true, nil
}
