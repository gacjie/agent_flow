# 系统环境

- 操作系统: windows/amd64
- Go 版本: go1.25.6
- Python: Python 3.12.10（命令: python）
- Node.js: v24.11.1
- npm: 11.6.2
- Shell: bash（所有命令通过 bash -c 执行）
- 工作目录: 由工具上下文自动设置，无需手动 cd

# 命令行规范

- 使用 bash 语法，不要使用 PowerShell 或 CMD 语法
- 路径分隔符统一使用 /（bash 在所有平台均接受）
- 使用 Unix 命令（ls、cat、grep、find、mkdir、rm 等）
- 不要使用 Windows 特有命令（dir、type、findstr、del、copy 等）
- 命令超时默认 30 秒，最大 300 秒
