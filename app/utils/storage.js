// 服务器管理：本地存储多个 AgentFlow 服务器连接信息
const SERVERS_KEY = 'af_servers'
const CURRENT_KEY = 'af_current_server'

export function getServers() {
  return uni.getStorageSync(SERVERS_KEY) || []
}

export function saveServers(servers) {
  uni.setStorageSync(SERVERS_KEY, servers)
}

export function getCurrentServer() {
  return uni.getStorageSync(CURRENT_KEY) || null
}

export function setCurrentServer(server) {
  uni.setStorageSync(CURRENT_KEY, server)
}

export function addServer(server) {
  const servers = getServers()
  server.id = Date.now().toString()
  servers.push(server)
  saveServers(servers)
  return server
}

export function updateServer(id, updates) {
  const servers = getServers()
  const idx = servers.findIndex(s => s.id === id)
  if (idx !== -1) {
    servers[idx] = { ...servers[idx], ...updates }
    saveServers(servers)
    const cur = getCurrentServer()
    if (cur && cur.id === id) setCurrentServer(servers[idx])
  }
}

export function removeServer(id) {
  const servers = getServers().filter(s => s.id !== id)
  saveServers(servers)
  const cur = getCurrentServer()
  if (cur && cur.id === id) {
    setCurrentServer(servers[0] || null)
  }
}
