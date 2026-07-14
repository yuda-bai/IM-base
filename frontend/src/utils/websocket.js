class ChatSocket {
  constructor() {
    this.ws = null
    this.listeners = []
    this.reconnectTimer = null
    this.reconnectDelay = 3000
    this.maxReconnectDelay = 30000
    this.isConnected = false
    this.intentionalClose = false
    this.pendingQueue = [] // 待发送的消息队列
  }

  connect(userId, userName) {
    if (this.ws) {
      const state = this.ws.readyState
      if (state === WebSocket.OPEN || state === WebSocket.CONNECTING) {
        console.log('WebSocket 已有活跃连接，跳过重复 connect')
        return
      }
    }

    this.userId = Number(userId)
    this.userName = userName
    this.intentionalClose = false

    // 开发环境：直连后端端口（VITE_WS_URL=ws://localhost:8080）
    // 生产环境：空字符串 → 使用当前页面地址（前后端同源）
    const wsBase = import.meta.env.VITE_WS_URL
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsHost = wsBase || `${protocol}//${window.location.host}`
    const wsUrl = `${wsHost}/user/SendUserMsg?userId=${this.userId}`

    try {
      this.ws = new WebSocket(wsUrl)
    } catch (e) {
      console.error('WebSocket 连接失败:', e)
      this.scheduleReconnect()
      return
    }

    const currentWs = this.ws

    this.ws.onopen = () => {
      if (currentWs !== this.ws) return
      console.log('✅ WebSocket 已连接, userId:', this.userId)
      this.isConnected = true
      this.reconnectDelay = 3000
      this.notifyListeners({
        type: 'system',
        content: '已连接到聊天服务器',
        timestamp: new Date().toISOString()
      })
       // ★ 连接恢复后发送所有待发消息
      if (this.pendingQueue.length > 0) {
        console.log(`📤 发送 ${this.pendingQueue.length} 条待发消息...`)
        const queue = [...this.pendingQueue]
        this.pendingQueue = []
        queue.forEach(msg => {
          this.sendMessage(msg.content, msg.type, msg.targetId, msg.messageId, msg.media, msg.pic)
        })
      }
    }

    this.ws.onmessage = (event) => {
      if (currentWs !== this.ws) return
      try {
        const raw = event.data
        const match = raw.match(/^\[ws\]\[(.+?)\]:\s*(.+)$/)
        const payload = match ? match[2] : raw
        const serverTime = match ? match[1] : new Date().toISOString()

        try {
          const data = JSON.parse(payload)
          const rawType = data.type ?? data.Type ?? 1
          const normalizedType = rawType === 'system' ? 'system' : (isNaN(Number(rawType)) ? rawType : Number(rawType))
          // ★ 修复：不能用 || null，因为 0 是有效值（系统消息 userId=0）
          const rawUserId = data.userId ?? data.FormId ?? data.formId
          const normalizedUserId = rawUserId != null ? Number(rawUserId) : null
          const rawTargetId = data.targetId ?? data.TargetId
          const normalizedTargetId = rawTargetId != null ? Number(rawTargetId) : null

          this.notifyListeners({
            userId: normalizedUserId,
            userName: data.userName ?? data.UserName ?? '未知用户',
            content: data.content ?? data.Content ?? '',
            type: normalizedType,
            media: String(data.media ?? data.Media ?? '1'),
            pic: data.pic ?? data.Pic ?? data.url ?? data.Url ?? '',
            targetId: normalizedTargetId,
            timestamp: serverTime,
            messageId: data.messageId || data.MessageId || `srv-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
            isOffline: !!(data.isOffline ?? data.is_offline ?? false)
          })
        } catch {
          this.notifyListeners({
            userId: 0,
            userName: '系统',
            content: payload,
            type: 'system',
            timestamp: serverTime,
            messageId: `sys-${Date.now()}`
          })
        }
      } catch (e) {
        console.error('消息解析失败:', e)
      }
    }

    this.ws.onerror = (error) => {
      if (currentWs !== this.ws) return
      console.error('WebSocket 错误:', error)
      this.notifyListeners({
        type: 'system',
        content: '连接出现错误',
        timestamp: new Date().toISOString()
      })
    }

    this.ws.onclose = (event) => {
      if (currentWs !== this.ws) return
      console.log('WebSocket 已断开:', event.code, event.reason)
      this.isConnected = false

      if (this.intentionalClose) {
        this.ws = null
        return
      }

      this.notifyListeners({
        type: 'system',
        content: '连接已断开，正在尝试重连...',
        timestamp: new Date().toISOString()
      })
      this.scheduleReconnect()
    }
  }

  sendMessage(content, type = 1, targetId, messageId, media = '1', pic = '') {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.error('WebSocket 未连接')
      this.pendingQueue.push({ content, type, targetId, messageId, media, pic })
      console.warn(`⚠️ WebSocket 未连接，消息已加入待发送队列 (队列长度: ${this.pendingQueue.length})`)
      return false
    }

    const payload = {
      userId: this.userId,
      FormId: this.userId,
      userName: this.userName,
      UserName: this.userName,
      content,
      Content: content,
      type,
      Type: type,
      media: media,
      Media: media,
      pic,
      Pic: pic,
      messageId: messageId || Date.now().toString(),
      MessageId: messageId || Date.now().toString()
    }

    if (targetId !== undefined && targetId !== null) {
      payload.targetId = Number(targetId)
      payload.TargetId = Number(targetId)
    }

    const message = JSON.stringify(payload)
    this.ws.send(message)
    return true
  }

  onMessage(callback) {
    this.listeners.push(callback)
    return () => {
      this.listeners = this.listeners.filter(fn => fn !== callback)
    }
  }

  notifyListeners(message) {
    this.listeners.forEach(fn => {
      try {
        fn(message)
      } catch (e) {
        console.error('消息监听器执行失败:', e)
      }
    })
  }

  scheduleReconnect() {
    if (this.reconnectTimer || this.intentionalClose) return

    console.log(`将在 ${this.reconnectDelay / 1000}s 后重连...`)
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      this.connect(this.userId, this.userName)
    }, this.reconnectDelay)

    this.reconnectDelay = Math.min(this.reconnectDelay * 1.5, this.maxReconnectDelay)
  }

  disconnect() {
    this.intentionalClose = true

    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }

    if (this.ws) {
      this.ws.onopen = null
      this.ws.onmessage = null
      this.ws.onerror = null
      this.ws.onclose = null
      this.ws.close(1000, '用户主动断开')
      this.ws = null
    }

    this.isConnected = false
    this.listeners = []
  }

  reconnect() {
    this.disconnect()
    this.reconnectDelay = 3000
    this.intentionalClose = false
    this.connect(this.userId, this.userName)
  }
}

export const chatSocket = new ChatSocket()
