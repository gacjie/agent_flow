package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"agent_flow/src/common"
	"os/exec"
	"strings"
	"time"
)

// RunCommandTool 执行 shell 命令
type RunCommandTool struct{}

func (t *RunCommandTool) Name() string { return "run_command" }

func (t *RunCommandTool) Description() string {
	return "在项目目录中执行 shell 命令。有超时限制和危险命令拦截。"
}

func (t *RunCommandTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "要执行的命令"},
			"timeout": {"type": "integer", "description": "超时秒数（默认 30，最大 300）"}
		},
		"required": ["command"]
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
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
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

	// 超时设置
	timeout := params.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	if timeout > 300 {
		timeout = 300
	}

	wc := GetWorkContext(ctx)
	if wc == nil {
		return ErrorResult("未设置工作目录上下文")
	}

	// 创建带超时的上下文
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	shell := common.GetShell()
	cmd := exec.CommandContext(execCtx, shell.Path, shell.Flag, params.Command)
	cmd.Dir = wc.WorkDir
	setProcGroup(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 在 goroutine 中执行，通过 select 实现可靠超时
	doneCh := make(chan error, 1)
	go func() {
		doneCh <- cmd.Run()
	}()

	var err error
	select {
	case err = <-doneCh:
	case <-execCtx.Done():
		killProcGroup(cmd)
		<-doneCh
		partialOutput := collectOutput(&stdout, &stderr)
		if partialOutput != "" {
			return ErrorResult(fmt.Sprintf("命令执行超时（%d秒）\n%s", timeout, partialOutput))
		}
		return ErrorResult(fmt.Sprintf("命令执行超时（%d秒）", timeout))
	}

	// 构建输出
	output := collectOutput(&stdout, &stderr)

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

func collectOutput(stdout, stderr *bytes.Buffer) string {
	var output strings.Builder

	if stdout.Len() > 0 {
		out := stdout.String()
		const maxOutput = 50000
		if len(out) > maxOutput {
			out = out[:maxOutput] + "\n... (输出已截断)"
		}
		output.WriteString(out)
	}

	if stderr.Len() > 0 {
		if output.Len() > 0 {
			output.WriteString("\n")
		}
		errOut := stderr.String()
		const maxStderr = 10000
		if len(errOut) > maxStderr {
			errOut = errOut[:maxStderr] + "\n... (错误输出已截断)"
		}
		output.WriteString("[stderr]\n" + errOut)
	}

	return output.String()
}
