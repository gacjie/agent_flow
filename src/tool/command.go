package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"agent_flow/src/common"
)

// bgProcess 后台进程信息
type bgProcess struct {
	PID       int
	Command   string
	Dir       string
	StartedAt time.Time
	LogFile   string
	cmd       *exec.Cmd
	done      chan struct{}
	exitErr   error
}

var bgRegistry = struct {
	sync.Mutex
	procs map[int]*bgProcess
}{procs: make(map[int]*bgProcess)}

// RunCommandTool 执行 shell 命令
type RunCommandTool struct{}

func (t *RunCommandTool) Name() string { return "run_command" }

func (t *RunCommandTool) Description() string {
	return "在项目目录中执行 shell 命令。支持超时限制、后台执行和进程管理。"
}

func (t *RunCommandTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "要执行的命令"},
			"timeout": {"type": "integer", "description": "超时秒数（默认 30，最大 300）"},
			"background": {"type": "boolean", "description": "后台运行命令，立即返回 PID，不等待命令结束"},
			"list_background": {"type": "boolean", "description": "列出所有后台进程及其运行状态"},
			"stop_background": {"type": "integer", "description": "停止指定 PID 的后台进程"}
		}
	}`)
}

// 危险命令前缀黑名单
var dangerousCommands = []string{
	"rm -rf /",
	"mkfs",
	"dd if=",
	":(){ :|:& };:",
	"> /dev/sda",
	"chmod -R 777 /",
	"shutdown",
	"reboot",
	"halt",
	"poweroff",
	"init 0",
	"init 6",
}

func (t *RunCommandTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		Command        string `json:"command"`
		Timeout        int    `json:"timeout"`
		Background     bool   `json:"background"`
		ListBackground bool   `json:"list_background"`
		StopBackground int    `json:"stop_background"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}

	if params.ListBackground {
		return t.listBackground()
	}

	if params.StopBackground > 0 {
		return t.stopBackground(params.StopBackground)
	}

	if params.Command == "" {
		return ErrorResult("命令不能为空")
	}

	// 检查危险命令
	cmdLower := strings.ToLower(strings.TrimSpace(params.Command))
	for _, dangerous := range dangerousCommands {
		if strings.Contains(cmdLower, dangerous) {
			return ErrorResult("拒绝执行危险命令: " + params.Command)
		}
	}

	wc := GetWorkContext(ctx)
	if wc == nil {
		return ErrorResult("未设置工作目录上下文")
	}

	if params.Background {
		return t.executeBackground(params.Command, wc)
	}

	// 正常执行（带超时）
	timeout := params.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	if timeout > 300 {
		timeout = 300
	}

	// 创建带超时的上下文
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	shell := common.GetShell()
	cmd := exec.CommandContext(execCtx, shell.Path, shell.Flag, params.Command)
	cmd.Dir = wc.WorkDir
	setProcGroup(cmd)

	// 用临时文件捕获输出（避免管道被 setsid/nohup 等子进程继承导致 cmd.Wait 永久阻塞）
	outFile, err := os.CreateTemp("", "cmd-out-*")
	if err != nil {
		return ErrorResult("创建临时文件失败: " + err.Error())
	}
	outName := outFile.Name()
	defer func() { outFile.Close(); os.Remove(outName) }()

	errFile, err := os.CreateTemp("", "cmd-err-*")
	if err != nil {
		return ErrorResult("创建临时文件失败: " + err.Error())
	}
	errName := errFile.Name()
	defer func() { errFile.Close(); os.Remove(errName) }()

	cmd.Stdout = outFile
	cmd.Stderr = errFile

	doneCh := make(chan error, 1)
	go func() {
		doneCh <- cmd.Run()
	}()

	select {
	case err = <-doneCh:
	case <-execCtx.Done():
		killProcGroup(cmd)
		select {
		case <-doneCh:
		case <-time.After(5 * time.Second):
		}
		partialOutput := collectFileOutput(outName, errName)
		if partialOutput != "" {
			return ErrorResult(fmt.Sprintf("命令执行超时（%d秒）\n%s", timeout, partialOutput))
		}
		return ErrorResult(fmt.Sprintf("命令执行超时（%d秒）", timeout))
	}

	output := collectFileOutput(outName, errName)

	if err != nil {
		if output != "" {
			return ErrorResult(fmt.Sprintf("命令执行失败: %s\n%s", err.Error(), output))
		}
		return ErrorResult("命令执行失败: " + err.Error())
	}

	if output == "" {
		return SuccessResult("命令执行成功（无输出）")
	}

	return SuccessResult(output)
}

func collectFileOutput(outPath, errPath string) string {
	var output strings.Builder

	if data, err := os.ReadFile(outPath); err == nil && len(data) > 0 {
		out := string(data)
		const maxOutput = 50000
		if len(out) > maxOutput {
			out = out[:maxOutput] + "\n... (输出已截断)"
		}
		output.WriteString(out)
	}

	if data, err := os.ReadFile(errPath); err == nil && len(data) > 0 {
		if output.Len() > 0 {
			output.WriteString("\n")
		}
		errOut := string(data)
		const maxStderr = 10000
		if len(errOut) > maxStderr {
			errOut = errOut[:maxStderr] + "\n... (错误输出已截断)"
		}
		output.WriteString("[stderr]\n" + errOut)
	}

	return output.String()
}

// executeBackground 后台执行命令，立即返回 PID
func (t *RunCommandTool) executeBackground(command string, wc *WorkContext) *Result {
	shell := common.GetShell()
	cmd := exec.Command(shell.Path, shell.Flag, command)
	cmd.Dir = wc.WorkDir
	setProcGroup(cmd)

	logFile, err := os.CreateTemp("", "bg-cmd-*.log")
	if err != nil {
		return ErrorResult("创建日志文件失败: " + err.Error())
	}

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		os.Remove(logFile.Name())
		return ErrorResult("启动后台命令失败: " + err.Error())
	}

	proc := &bgProcess{
		PID:       cmd.Process.Pid,
		Command:   command,
		Dir:       wc.WorkDir,
		StartedAt: time.Now(),
		LogFile:   logFile.Name(),
		cmd:       cmd,
		done:      make(chan struct{}),
	}

	bgRegistry.Lock()
	bgRegistry.procs[cmd.Process.Pid] = proc
	bgRegistry.Unlock()

	go func() {
		proc.exitErr = cmd.Wait()
		logFile.Close()
		close(proc.done)
	}()

	return SuccessResult(fmt.Sprintf("进程已在后台启动\nPID: %d\n日志: %s", cmd.Process.Pid, logFile.Name()))
}

// listBackground 列出所有后台进程
func (t *RunCommandTool) listBackground() *Result {
	bgRegistry.Lock()
	defer bgRegistry.Unlock()

	// 清理已退出超过 30 分钟的条目
	cutoff := time.Now().Add(-30 * time.Minute)
	for pid, proc := range bgRegistry.procs {
		select {
		case <-proc.done:
			if proc.StartedAt.Before(cutoff) {
				os.Remove(proc.LogFile)
				delete(bgRegistry.procs, pid)
			}
		default:
		}
	}

	if len(bgRegistry.procs) == 0 {
		return SuccessResult("没有后台进程")
	}

	// 按启动时间排序
	procs := make([]*bgProcess, 0, len(bgRegistry.procs))
	for _, p := range bgRegistry.procs {
		procs = append(procs, p)
	}
	sort.Slice(procs, func(i, j int) bool {
		return procs[i].StartedAt.Before(procs[j].StartedAt)
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("后台进程（共 %d 个）：\n", len(procs)))
	for _, proc := range procs {
		status := "运行中"
		select {
		case <-proc.done:
			if proc.exitErr != nil {
				status = "已退出（" + proc.exitErr.Error() + "）"
			} else {
				status = "已退出（正常）"
			}
		default:
		}
		sb.WriteString(fmt.Sprintf("\n  PID: %d  [%s]  启动: %s\n  命令: %s\n  目录: %s\n  日志: %s\n",
			proc.PID, status, proc.StartedAt.Format("15:04:05"),
			common.TruncateString(proc.Command, 120), proc.Dir, proc.LogFile))
	}
	return SuccessResult(sb.String())
}

// stopBackground 停止指定 PID 的后台进程
func (t *RunCommandTool) stopBackground(pid int) *Result {
	bgRegistry.Lock()
	proc, ok := bgRegistry.procs[pid]
	bgRegistry.Unlock()

	if !ok {
		return ErrorResult(fmt.Sprintf("未找到 PID %d 的后台进程", pid))
	}

	select {
	case <-proc.done:
		return SuccessResult(fmt.Sprintf("进程 %d 已经退出", pid))
	default:
	}

	killProcGroup(proc.cmd)

	select {
	case <-proc.done:
	case <-time.After(5 * time.Second):
	}

	return SuccessResult(fmt.Sprintf("进程 %d 已停止", pid))
}
