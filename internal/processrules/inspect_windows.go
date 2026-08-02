//go:build windows

package processrules

import (
	"errors"
	"sort"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

type process struct {
	name, path string
	pathError  error
}

func Inspect(identities []Identity) ([]Status, error) {
	processes, err := enumerate()
	if err != nil {
		return nil, err
	}
	result := make([]Status, 0, len(identities))
	for _, identity := range identities {
		result = append(result, inspect(identity, processes))
	}
	return result, nil
}

func enumerate() ([]process, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{}
	entry.Size = uint32(unsafe.Sizeof(entry))
	result := []process{}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		name := strings.ToLower(windows.UTF16ToString(entry.ExeFile[:]))
		path, pathErr := processPath(entry.ProcessID)
		result = append(result, process{name: name, path: path, pathError: pathErr})
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, err
	}
	return result, nil
}

func processPath(pid uint32) (string, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buffer[:size]), nil
}

func inspect(identity Identity, processes []process) Status {
	status := Status{Type: identity.Type, Value: identity.Value, State: "not-running", Message: "当前未发现匹配进程；进程启动后规则将按稳定标识匹配"}
	wanted := strings.ToLower(strings.TrimSpace(identity.Value))
	paths := map[string]struct{}{}
	denied := 0
	for _, process := range processes {
		matched := false
		if identity.Type == "process-name" {
			matched = process.name == wanted
		} else if identity.Type == "process-path" && process.path != "" {
			matched = strings.EqualFold(process.path, wanted)
		}
		if identity.Type == "process-path" && process.pathError != nil {
			denied++
		}
		if !matched {
			continue
		}
		status.Matches++
		if process.path != "" {
			paths[process.path] = struct{}{}
		}
	}
	for path := range paths {
		status.Paths = append(status.Paths, path)
	}
	sort.Strings(status.Paths)
	if status.Matches > 0 {
		status.State, status.Message = "matched", "当前进程身份已识别"
	}
	if identity.Type == "process-name" && len(paths) > 1 {
		status.State, status.Message = "ambiguous", "发现同名程序位于不同路径；请改用进程完整路径"
	}
	if identity.Type == "process-path" && status.Matches == 0 && denied > 0 {
		status.State, status.Message = "permission-limited", "部分进程路径因权限不足无法核对；当前规则不会静默改写为名称匹配"
	}
	return status
}
