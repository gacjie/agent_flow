// 消息解析工具函数 — 将原始消息转为 block 模型

/**
 * 截断文本
 */
export function truncate(str, len = 60) {
  if (!str) return ''
  return str.length > len ? str.slice(0, len) + '...' : str
}

/**
 * 格式化工具参数为可读 JSON
 */
export function formatArgs(args) {
  if (!args) return ''
  if (typeof args === 'string') {
    try { return JSON.stringify(JSON.parse(args), null, 2) } catch { return args }
  }
  try { return JSON.stringify(args, null, 2) } catch { return String(args) }
}

/**
 * 解析内容文本为 block 数组（处理 <think> 标签和代码围栏）
 */
export function parseContentBlocks(text) {
  if (!text) return []
  const blocks = []
  let remaining = text

  while (remaining.length > 0) {
    // 查找最近的特殊标记
    const thinkIdx = remaining.indexOf('<think>')
    const codeIdx = remaining.indexOf('```')

    // 找到最近的标记位置
    let nearest = -1
    let nearestType = null
    if (thinkIdx >= 0 && (codeIdx < 0 || thinkIdx < codeIdx)) {
      nearest = thinkIdx; nearestType = 'think'
    } else if (codeIdx >= 0) {
      nearest = codeIdx; nearestType = 'code'
    }

    if (nearest < 0) {
      // 没有特殊标记，剩余全部为文本
      if (remaining.trim()) blocks.push({ type: 'text', content: remaining })
      break
    }

    // 标记前的文本
    if (nearest > 0) {
      const before = remaining.slice(0, nearest)
      if (before.trim()) blocks.push({ type: 'text', content: before })
    }

    if (nearestType === 'think') {
      const closeIdx = remaining.indexOf('</think>', nearest + 7)
      if (closeIdx >= 0) {
        const thinkContent = remaining.slice(nearest + 7, closeIdx)
        if (thinkContent.trim()) blocks.push({ type: 'reasoning', content: thinkContent.trim(), collapsed: true })
        remaining = remaining.slice(closeIdx + 8)
      } else {
        // 未闭合，剩余都是 thinking
        const thinkContent = remaining.slice(nearest + 7)
        if (thinkContent.trim()) blocks.push({ type: 'reasoning', content: thinkContent.trim(), collapsed: true })
        break
      }
    } else if (nearestType === 'code') {
      // 解析代码围栏
      const afterFence = remaining.slice(nearest + 3)
      const lineEnd = afterFence.indexOf('\n')
      const lang = lineEnd >= 0 ? afterFence.slice(0, lineEnd).trim() : ''
      const codeStart = lineEnd >= 0 ? lineEnd + 1 : 0
      const closeIdx = afterFence.indexOf('```', codeStart)
      if (closeIdx >= 0) {
        const codeContent = afterFence.slice(codeStart, closeIdx)
        blocks.push({ type: 'code', lang, content: codeContent.replace(/\n$/, '') })
        remaining = afterFence.slice(closeIdx + 3)
      } else {
        // 未闭合代码围栏
        const codeContent = afterFence.slice(codeStart)
        blocks.push({ type: 'code', lang, content: codeContent })
        break
      }
    }
  }
  return blocks
}

/**
 * 解析 tool_calls 字段
 */
function parseToolCalls(raw) {
  if (!raw) return null
  if (Array.isArray(raw)) return raw
  try { return JSON.parse(raw) } catch { return null }
}

/**
 * 将 API 返回的原始消息数组转为 block 模型
 */
export function parseHistoryMessages(rawMessages) {
  if (!rawMessages || !rawMessages.length) return []
  const result = []

  for (const m of rawMessages) {
    if (m.role === 'system') continue

    if (m.role === 'tool') {
      // tool result 追加到前一条 assistant 消息
      const isError = (m.content || '').startsWith('[错误]') || m.is_error
      const block = { type: 'tool_result', content: m.content || '', isError: !!isError, collapsed: true }
      const prev = [...result].reverse().find(msg => msg.role === 'assistant')
      if (prev) { prev.blocks.push(block) }
      else { result.push({ role: 'assistant', blocks: [block] }) }
      continue
    }

    const msg = { role: m.role, blocks: [] }

    if (m.role === 'assistant') {
      // 思考内容
      if (m.reasoning_content) {
        msg.blocks.push({ type: 'reasoning', content: m.reasoning_content, collapsed: true })
      }
      // 主内容（解析 think 标签和代码围栏）
      if (m.content) {
        msg.blocks.push(...parseContentBlocks(m.content))
      }
      // 工具调用
      const toolCalls = parseToolCalls(m.tool_calls)
      if (toolCalls && toolCalls.length) {
        for (const tc of toolCalls) {
          msg.blocks.push({
            type: 'tool_call',
            name: tc.name || tc.function?.name || '未知工具',
            args: formatArgs(tc.arguments || tc.args || tc.function?.arguments),
            collapsed: true
          })
        }
      }
    } else {
      // user 消息
      msg.blocks.push({ type: 'text', content: m.content || '' })
    }

    if (msg.blocks.length > 0) result.push(msg)
  }
  return result
}
