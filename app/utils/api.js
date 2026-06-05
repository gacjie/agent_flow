import { getCurrentServer } from './storage.js'

function getBase() {
  const s = getCurrentServer()
  if (!s) throw new Error('未选择服务器')
  return { baseURL: s.baseURL, token: s.token }
}

function headers(token) {
  return { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` }
}

export function request(method, path, data) {
  const { baseURL, token } = getBase()
  return new Promise((resolve, reject) => {
    uni.request({
      url: baseURL + path,
      method,
      data,
      header: headers(token),
      success: res => res.statusCode < 400 ? resolve(res.data) : reject(res.data),
      fail: err => reject(err)
    })
  })
}

// APP 登录（返回 API Token）
export function apiLogin(baseURL, username, password) {
  return new Promise((resolve, reject) => {
    uni.request({
      url: baseURL + '/api/auth/login',
      method: 'POST',
      data: { username, password },
      header: { 'Content-Type': 'application/json' },
      success: res => res.statusCode === 200 ? resolve(res.data.data) : reject(res.data),
      fail: err => reject(err)
    })
  })
}

export function apiLogout() {
  return request('POST', '/api/auth/logout')
}

// 工作区
export function listWorkspaces() {
  return request('GET', '/admin/workbench/workspaces-all')
}

// 会话
export function listConversations(workspaceId) {
  return request('GET', `/admin/workbench/conversations?workspace_id=${workspaceId}`)
}

export function createConversation(workspaceId, agentId) {
  return request('POST', '/admin/workbench/conversations', { workspace_id: workspaceId, agent_id: agentId })
}

export function getMessages(convId, workspaceId) {
  return request('GET', `/admin/workbench/conversations/${convId}/messages?workspace_id=${workspaceId}`)
}

export function stopConversation(convId, workspaceId) {
  return request('POST', `/admin/workbench/conversations/${convId}/stop?workspace_id=${workspaceId}`, {})
}

// 任务
export function listTasks(workspaceId) {
  return request('GET', `/admin/workbench/tasks?workspace_id=${workspaceId}`)
}

// SSE 解析公用函数
function parseSSEChunk(chunk, onEvent, onDone) {
  chunk.split('\n').forEach(line => {
    if (!line.startsWith('data: ')) return
    if (line === 'data: [DONE]') { onDone && onDone(); return }
    try { onEvent && onEvent(JSON.parse(line.slice(6))) } catch {}
  })
}

// SSE 流式对话（多平台支持）
export function sendMessageSSE({ baseURL, token, convId, workspaceId, message, lastSeq, onEvent, onDone, onError }) {
  const url = `${baseURL}/admin/workbench/conversations/${convId}/send`
  const body = JSON.stringify({ message, workspace_id: workspaceId, last_seq: lastSeq || 0 })

  // #ifdef APP-PLUS
  const xhr = new plus.net.XMLHttpRequest()
  xhr.open('POST', url)
  xhr.setRequestHeader('Authorization', `Bearer ${token}`)
  xhr.setRequestHeader('Content-Type', 'application/json')
  let offset = 0
  xhr.onreadystatechange = () => {
    if (xhr.readyState >= 3) {
      const chunk = xhr.responseText.slice(offset)
      offset = xhr.responseText.length
      parseSSEChunk(chunk, onEvent, onDone)
      if (xhr.readyState === 4) onDone && onDone()
    }
  }
  xhr.onerror = () => onError && onError()
  xhr.send(body)
  return xhr
  // #endif

  // #ifdef H5
  const xhr2 = new XMLHttpRequest()
  xhr2.open('POST', url)
  xhr2.setRequestHeader('Authorization', `Bearer ${token}`)
  xhr2.setRequestHeader('Content-Type', 'application/json')
  let off = 0
  xhr2.onprogress = () => {
    const chunk = xhr2.responseText.slice(off)
    off = xhr2.responseText.length
    parseSSEChunk(chunk, onEvent, onDone)
  }
  xhr2.onload = () => onDone && onDone()
  xhr2.onerror = () => onError && onError()
  xhr2.send(body)
  return xhr2
  // #endif
}
