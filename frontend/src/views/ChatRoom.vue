<template>
  <div class="chat-container">
    <!-- 侧边栏 -->
    <Sidebar
      :user-list="userStore.userList"
      :friend-list="userStore.friendList"
      :online-ids="userStore.onlineUserIds"
      :current-user="userStore.currentUser"
      :active-friend-id="selectedFriend?.ID"
      @logout="handleLogout"
      @edit-profile="showProfile = true"
      @refresh-users="loadUsers"
      @refresh-friends="loadFriends"
      @add-friend="handleAddFriend"
      @select-friend="handleSelectFriend"
    />

    <!-- 主聊天区域 -->
    <main class="chat-main">
      <ChatArea
        ref="chatAreaRef"
        v-if="userStore.currentUser && userStore.currentUser.ID != null"
        :messages="displayMessages"
        :current-user="userStore.currentUser"
        :selected-friend="selectedFriend"
        :connection-status="connectionStatus"
        :has-more="hasMore"
        :is-loading-more="isLoadingMore"
        @send="handleSendMessage"
        @send-image="handleSendImage"
        @send-voice="handleSendVoice"
        @reconnect="handleReconnect"
        @load-more="handleLoadMore"
      />
      <div v-else class="chat-placeholder">
        <div class="placeholder-icon">⏳</div>
        <p>正在加载用户信息...</p>
        <el-button @click="router.push('/login')">返回登录</el-button>
      </div>
    </main>

    <!-- 个人信息编辑弹窗 -->
    <UserProfile
      v-if="showProfile"
      :user="userStore.currentUser"
      @close="showProfile = false"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '../stores/user'
import { chatSocket } from '../utils/websocket'
import { uploadImage, uploadVoice, getChatRecord, sendMessageHttp } from '../api/user'
import Sidebar from '../components/Sidebar.vue'
import ChatArea from '../components/ChatArea.vue'
import UserProfile from '../components/UserProfile.vue'

const router = useRouter()
const userStore = useUserStore()

const messages = ref([])
const selectedFriend = ref(null)
const showProfile = ref(false)
const connectionStatus = ref('connecting')
const chatAreaRef = ref(null)
let removeMessageListener = null

// 分页状态
const currentPage = ref(1)
const hasMore = ref(true)
const isLoadingMore = ref(false)
const pageSize = 20

function normalizeId(id) {
  if (id === undefined || id === null || id === '') return null
  const numericValue = Number(id)
  return Number.isNaN(numericValue) ? id : numericValue
}

const displayMessages = computed(() => {
  // 未选择好友时显示所有消息（聊天室模式）
  if (!selectedFriend.value) {
    return messages.value
  }

  const currentUserId = normalizeId(userStore.currentUser?.ID ?? userStore.currentUser?.id)
  const selectedFriendId = normalizeId(selectedFriend.value?.ID ?? selectedFriend.value?.id)

  // 如果无法确定当前用户或好友 ID，放弃过滤，返回所有消息
  if (currentUserId === null || selectedFriendId === null) {
    console.warn('displayMessages: 用户ID或好友ID为null，显示所有消息', { currentUserId, selectedFriendId })
    return messages.value
  }

  const filtered = messages.value.filter(msg => {
    // 系统消息始终显示（用 == 兼容字符串 "0" 和数字 0）
    const msgType = msg.type ?? msg.Type
    if (msgType === 'system' || msgType == 3 || msgType == 0) {
      return true
    }

    // 兼容多种字段名获取消息的发送者和目标
    const msgUserId = normalizeId(msg.userId ?? msg.FormId ?? msg.formId ?? msg.UserId ?? msg.senderId)
    const msgTargetId = normalizeId(msg.targetId ?? msg.TargetId ?? msg.TargetID ?? msg.targetID)

    // 双方 ID 都缺失时保留消息（可能是格式异常，至少显示出来）
    if (msgUserId === null && msgTargetId === null) {
      return true
    }

    const involvedWithCurrentUser = msgUserId === currentUserId || msgTargetId === currentUserId
    const involvedWithSelectedFriend = msgUserId === selectedFriendId || msgTargetId === selectedFriendId

    return involvedWithCurrentUser && involvedWithSelectedFriend
  })

  return filtered
})

function saveMessagesToStorage() {
  if (!userStore.currentUser?.ID) return
  try {
    localStorage.setItem(`ginchat_messages_${userStore.currentUser.ID}`, JSON.stringify(messages.value))
  } catch (e) {
    console.warn('保存消息到本地存储失败:', e)
  }
}

function loadMessagesFromStorage() {
  if (!userStore.currentUser?.ID) {
    console.warn('无法加载消息：当前用户 ID 为空')
    return
  }
  try {
    const saved = localStorage.getItem(`ginchat_messages_${userStore.currentUser.ID}`)
    if (saved) {
      const parsed = JSON.parse(saved)
      if (Array.isArray(parsed) && parsed.length > 0) {
        // 规范化从存储中加载的消息，确保字段名一致
        messages.value = parsed.map(msg => ({
          userId: normalizeId(msg.userId ?? msg.FormId ?? msg.formId ?? msg.UserId ?? msg.senderId ?? null),
          userName: msg.userName ?? msg.UserName ?? msg.user_name ?? msg.senderName ?? '',
          content: msg.content ?? msg.Content ?? msg.text ?? '',
          type: msg.type ?? msg.Type ?? 1,
          media: String(msg.media ?? msg.Media ?? '1'),
          pic: msg.pic ?? msg.Pic ?? msg.url ?? msg.Url ?? '',
          audioUrl: msg.audioUrl ?? msg.audio ?? msg.Audio ?? msg.pic ?? msg.Pic ?? msg.url ?? msg.Url ?? '',
          duration: Number(msg.duration ?? msg.Duration ?? msg.dur ?? 0),
          targetId: normalizeId(msg.targetId ?? msg.TargetId ?? msg.TargetID ?? null),
          timestamp: msg.timestamp ?? msg.Timestamp ?? msg.time ?? '',
          messageId: msg.messageId ?? msg.MessageId ?? msg.id ?? msg._id ?? `${msg.timestamp || Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
          isOffline: Boolean(msg.isOffline ?? msg.is_offline ?? msg.IsOffline ?? false),
          pending: Boolean(msg.pending ?? false)
        }))
        console.log(`从本地存储加载了 ${messages.value.length} 条消息`)
      } else if (Array.isArray(parsed)) {
        console.log('本地存储中无历史消息')
        messages.value = []
      }
    } else {
      console.log('本地存储中无消息记录')
      messages.value = []
    }
  } catch (e) {
    console.warn('读取历史消息失败:', e)
    messages.value = []
  }
}

onMounted(() => {
  // 确保用户已正确恢复
  const restoredUser = userStore.restoreCurrentUser?.()
  if (!restoredUser || restoredUser.ID == null) {
    console.warn('未找到有效用户，跳转到登录页')
    router.push('/login')
    return
  }

  // 确保 userStore.currentUser 已正确设置
  if (!userStore.currentUser || userStore.currentUser.ID == null) {
    userStore.currentUser = restoredUser
  }

  loadUsers()
  loadFriends()

  const user = userStore.currentUser
  if (!user || user.ID == null) {
    router.push('/login')
    return
  }

  if (removeMessageListener) {
    removeMessageListener()
    removeMessageListener = null
  }

  // 先断开旧连接（如果有），再连接
  chatSocket.disconnect()
  chatSocket.connect(user.ID, user.Name)

  connectionStatus.value = chatSocket.isConnected ? 'connected' : 'connecting'

  removeMessageListener = chatSocket.onMessage((msg) => {
    console.log('WebSocket received message:', msg)
    if (msg.type === 'system') {
      if (msg.content.includes('已连接')) {
        connectionStatus.value = 'connected'
      } else if (msg.content.includes('断开')) {
        connectionStatus.value = 'disconnected'
      }
    }

    const normalizedMsg = {
      userId: normalizeId(msg.userId ?? msg.FormId ?? msg.formId ?? msg.UserId ?? null),
      targetId: normalizeId(msg.targetId ?? msg.TargetId ?? msg.TargetID ?? null),
      type: msg.type ?? msg.Type ?? 1,
      content: msg.content ?? msg.Content ?? '',
      userName: msg.userName ?? msg.UserName ?? msg.user_name ?? '',
      media: String(msg.media ?? msg.Media ?? '1'),
      pic: msg.pic ?? msg.Pic ?? msg.url ?? msg.Url ?? '',
      audioUrl: msg.audioUrl ?? msg.audio ?? msg.Audio ?? msg.pic ?? msg.Pic ?? msg.url ?? msg.Url ?? '',
      duration: Number(msg.duration ?? msg.Duration ?? msg.dur ?? 0),
      timestamp: msg.timestamp ?? msg.Timestamp ?? new Date().toISOString(),
      messageId: msg.messageId ?? msg.MessageId ?? msg.id ?? `srv-${Date.now()}`,
      isOffline: !!(msg.isOffline ?? msg.is_offline ?? msg.IsOffline ?? false)
    }

    // 查找是否有相同 messageId 的本地消息（统一转为字符串比较，避免数字 vs 字符串不匹配）
    const existingIndex = messages.value.findIndex(item =>
      String(item.messageId) === String(normalizedMsg.messageId)
    )
    if (existingIndex >= 0) {
      const existing = messages.value[existingIndex]
      // 只更新服务器确认的字段，保留本地已有的有效值
      messages.value[existingIndex] = {
        ...existing,
        // 收到服务器回显，说明发送成功，去掉 pending 标记
        pending: false,
        // 只有当服务器返回了有效内容时才更新
        content: normalizedMsg.content || existing.content || '',
        // 更新为服务器时间戳
        timestamp: normalizedMsg.timestamp || existing.timestamp,
        // 离线消息标记
        isOffline: normalizedMsg.isOffline,
        // 图片/语音相关字段
        media: normalizedMsg.media || existing.media || '1',
        pic: normalizedMsg.pic || existing.pic || '',
        audioUrl: normalizedMsg.audioUrl || existing.audioUrl || '',
        duration: normalizedMsg.duration || existing.duration || 0,
        // 服务器可能规范化了 ID
        userId: normalizedMsg.userId != null ? normalizedMsg.userId : existing.userId,
        targetId: normalizedMsg.targetId != null ? normalizedMsg.targetId : existing.targetId
      }
      console.log('更新已发送消息:', normalizedMsg.messageId)
    } else {
      // 新消息（可能是别人发的，或是服务器转发的）
      messages.value.push(normalizedMsg)
      console.log('新增收到消息:', normalizedMsg.messageId)
    }

    saveMessagesToStorage()
    console.log('messages length after receive:', messages.value.length)
  })
})

onUnmounted(() => {
  if (removeMessageListener) {
    removeMessageListener()
  }
})

async function loadUsers() {
  await userStore.fetchUserList()
}

async function loadFriends() {
  await userStore.fetchFriendList()
}

async function handleSendMessage(content) {
  const user = userStore.currentUser
  if (!user || user.ID == null) {
    connectionStatus.value = 'disconnected'
    return
  }

  const targetId = selectedFriend.value?.ID ?? selectedFriend.value?.id
  if (targetId == null) {
    alert('请选择一个好友后再发送消息')
    return
  }

  const messageId = `msg-${Date.now()}`
  const normalizedTargetId = normalizeId(targetId)
  const normalizedUserId = normalizeId(user.ID)

  const localMsg = {
    userId: normalizedUserId,
    userName: user.Name,
    content,
    type: 1,
    media: '1',
    pic: '',
    audioUrl: '',
    duration: 0,
    timestamp: new Date().toISOString(),
    messageId,
    targetId: normalizedTargetId,
    isOffline: false,
    pending: true
  }
  messages.value.push(localMsg)
  saveMessagesToStorage()
  console.log('local message added:', localMsg)

  // 优先用 HTTP POST 发送（可靠），WebSocket 仅做备用
  try {
    const res = await sendMessageHttp({
      FormId: normalizedUserId,
      targetId: normalizedTargetId,
      content,
      type: 1,
      media: '1',
      pic: ''
    })
    console.log('HTTP send success:', res)
    // 更新本地消息状态
    const idx = messages.value.findIndex(m => m.messageId === messageId)
    if (idx >= 0) {
      messages.value[idx].pending = false
      if (res.data?.ID) {
        messages.value[idx].serverId = res.data.ID
      }
    }
    saveMessagesToStorage()
  } catch (e) {
    console.warn('HTTP send failed, trying WebSocket fallback:', e)
    // 备用：WebSocket 发送
    const sent = chatSocket.sendMessage(content, 1, normalizedTargetId, messageId, '1', '')
    if (!sent) {
      connectionStatus.value = 'disconnected'
    }
  }
}

async function handleSendImage(file, callbacks) {
  const user = userStore.currentUser
  if (!user || user.ID == null) {
    connectionStatus.value = 'disconnected'
    callbacks?.onError?.('未登录')
    return
  }

  const targetId = selectedFriend.value?.ID ?? selectedFriend.value?.id
  if (targetId == null) {
    alert('请选择一个好友后再发送消息')
    callbacks?.onError?.('未选择好友')
    return
  }

  // 通知 ChatArea 开始上传
  callbacks?.onStart?.()

  // 步骤1：上传图片到服务器
  let imageUrl = ''
  try {
    const res = await uploadImage(file)
    if (res.code === 0 && res.data?.imagUrl) {
      imageUrl = res.data.imagUrl
      console.log('图片上传成功:', imageUrl)
    } else {
      throw new Error(res.message || '上传失败')
    }
  } catch (err) {
    console.error('图片上传失败:', err)
    callbacks?.onError?.(err.message || '图片上传失败，请重试')
    return
  }

  // 步骤2：通过 HTTP 发送图片消息（可靠）
  const messageId = `img-${Date.now()}`
  const normalizedTargetId = normalizeId(targetId)
  const normalizedUserId = normalizeId(user.ID)

  const localMsg = {
    userId: normalizedUserId,
    userName: user.Name,
    content: '[图片]',
    type: 1,
    media: '3',
    pic: imageUrl,
    timestamp: new Date().toISOString(),
    messageId,
    targetId: normalizedTargetId,
    isOffline: false,
    pending: true
  }
  messages.value.push(localMsg)
  saveMessagesToStorage()

  try {
    await sendMessageHttp({ FormId: normalizedUserId, targetId: normalizedTargetId, content: '[图片]', type: 1, media: '3', pic: imageUrl })
    const idx = messages.value.findIndex(m => m.messageId === messageId)
    if (idx >= 0) messages.value[idx].pending = false
    saveMessagesToStorage()
  } catch (e) {
    console.warn('Image HTTP send failed, WS fallback:', e)
    chatSocket.sendMessage('[图片]', 1, normalizedTargetId, messageId, '3', imageUrl)
  }

  console.log('sending image message', { imageUrl, targetId: normalizedTargetId, messageId })
  callbacks?.onDone?.()
}

async function handleSendVoice(file, callbacks) {
  const user = userStore.currentUser
  if (!user || user.ID == null) {
    connectionStatus.value = 'disconnected'
    callbacks?.onError?.('未登录')
    return
  }

  const targetId = selectedFriend.value?.ID ?? selectedFriend.value?.id
  if (targetId == null) {
    alert('请选择一个好友后再发送消息')
    callbacks?.onError?.('未选择好友')
    return
  }

  // 通知 ChatArea 开始上传
  callbacks?.onStart?.()

  // 步骤1：上传语音到服务器
  let audioUrl = ''
  try {
    const res = await uploadVoice(file)
    if (res.code === 0 && res.data?.audio) {
      audioUrl = res.data.audio
      console.log('语音上传成功:', audioUrl)
    } else {
      throw new Error(res.message || '上传失败')
    }
  } catch (err) {
    console.error('语音上传失败:', err)
    callbacks?.onError?.(err.message || '语音上传失败，请重试')
    return
  }

  // 步骤2：通过 HTTP 发送语音消息（可靠）
  const messageId = `voice-${Date.now()}`
  const normalizedTargetId = normalizeId(targetId)
  const normalizedUserId = normalizeId(user.ID)
  const duration = callbacks?.duration || 0

  const localMsg = {
    userId: normalizedUserId,
    userName: user.Name,
    content: '[语音]',
    type: 1,
    media: '4',
    pic: audioUrl,
    audioUrl,
    duration,
    timestamp: new Date().toISOString(),
    messageId,
    targetId: normalizedTargetId,
    isOffline: false,
    pending: true
  }
  messages.value.push(localMsg)
  saveMessagesToStorage()

  try {
    await sendMessageHttp({ FormId: normalizedUserId, targetId: normalizedTargetId, content: '[语音]', type: 1, media: '4', pic: audioUrl })
    const idx = messages.value.findIndex(m => m.messageId === messageId)
    if (idx >= 0) messages.value[idx].pending = false
    saveMessagesToStorage()
  } catch (e) {
    console.warn('Voice HTTP send failed, WS fallback:', e)
    chatSocket.sendMessage('[语音]', 1, normalizedTargetId, messageId, '4', audioUrl)
  }

  console.log('sending voice message', { audioUrl, targetId: normalizedTargetId, messageId, duration })

  // 通知 ChatArea 上传完成
  callbacks?.onDone?.()
}

async function handleSelectFriend(friend) {
  // 使用 store 的统一规范化函数，确保 friend 对象包含 ID、Name 等标准字段
  if (friend && userStore.normalizeContact) {
    selectedFriend.value = userStore.normalizeContact(friend) || friend
  } else {
    selectedFriend.value = friend
  }

  // 重置分页状态
  currentPage.value = 1
  hasMore.value = true
  isLoadingMore.value = false

  // 清空旧消息
  messages.value = []

  // 从服务端加载第一页历史记录
  await loadChatHistory()

  // 第一页加载后滚动到底部
  nextTick(() => {
    chatAreaRef.value?.scrollToBottom?.()
  })
}

// 将服务端 Message 格式转换为前端统一格式
function normalizeMsgFromServer(msg) {
  return {
    userId: normalizeId(msg.FormId ?? msg.formId ?? msg.userId ?? null),
    targetId: normalizeId(msg.TargetId ?? msg.targetId ?? msg.TargetID ?? null),
    type: msg.Type ?? msg.type ?? 1,
    content: msg.Content ?? msg.content ?? '',
    media: String(msg.Media ?? msg.media ?? '1'),
    pic: msg.Pic ?? msg.pic ?? msg.Url ?? msg.url ?? '',
    audioUrl: msg.Pic ?? msg.pic ?? msg.Url ?? msg.url ?? '',
    duration: 0,
    userName: msg.UserName ?? msg.userName ?? '',
    timestamp: msg.CreatedAt ?? msg.createdAt ?? msg.timestamp ?? '',
    messageId: String(msg.ID ?? msg.id ?? msg.messageId ?? ''),
    isOffline: !!(msg.IsOffline ?? msg.isOffline ?? msg.is_offline ?? false),
    pending: false
  }
}

async function loadChatHistory() {
  const user = userStore.currentUser
  const friend = selectedFriend.value
  if (!user?.ID || !friend?.ID) return

  isLoadingMore.value = true

  try {
    const res = await getChatRecord(user.ID, friend.ID, currentPage.value, pageSize)
    if (res.code === 0) {
      // 兼容两种响应格式：data.list（分页）或 data 本身是数组
      const data = res.data
      const list = data?.list ?? data ?? []
      const total = data?.total ?? 0

      const normalized = (Array.isArray(list) ? list : []).map(normalizeMsgFromServer)

      if (currentPage.value === 1) {
        // 第一页：替换
        messages.value = normalized
      } else {
        // 后续页：头部插入（旧消息在前）
        messages.value = [...normalized, ...messages.value]
      }

      // 判断是否还有更多
      if (total > 0) {
        hasMore.value = messages.value.length < total
      } else {
        hasMore.value = normalized.length >= pageSize
      }

      console.log(`聊天记录加载完成: 第${currentPage.value}页, ${normalized.length}条, 总计${messages.value.length}条, hasMore=${hasMore.value}`)
    }
  } catch (err) {
    console.error('加载聊天记录失败:', err)
  } finally {
    isLoadingMore.value = false
  }
}

async function handleLoadMore({ prevScrollHeight }) {
  if (!hasMore.value || isLoadingMore.value) return

  currentPage.value++

  try {
    const user = userStore.currentUser
    const friend = selectedFriend.value
    if (!user?.ID || !friend?.ID) return

    isLoadingMore.value = true

    const res = await getChatRecord(user.ID, friend.ID, currentPage.value, pageSize)
    if (res.code === 0) {
      const data = res.data
      const list = data?.list ?? data ?? []
      const total = data?.total ?? 0

      const normalized = (Array.isArray(list) ? list : []).map(normalizeMsgFromServer)

      if (normalized.length > 0) {
        // 头部插入旧消息
        messages.value = [...normalized, ...messages.value]
      }

      if (total > 0) {
        hasMore.value = messages.value.length < total
      } else {
        hasMore.value = normalized.length >= pageSize
      }

      // 恢复滚动位置
      await nextTick()
      const el = chatAreaRef.value?.messageList
      if (el) {
        el.scrollTop = el.scrollHeight - prevScrollHeight
      }
    }
  } catch (err) {
    console.error('加载更多消息失败:', err)
  } finally {
    isLoadingMore.value = false
  }
}

function handleReconnect() {
  const user = userStore.currentUser
  if (!user || user.ID == null) {
    router.push('/login')
    return
  }
  connectionStatus.value = 'connecting'
  // 确保 reconnect 时使用正确的用户凭据
  chatSocket.disconnect()
  chatSocket.connect(user.ID, user.Name)
}

async function handleAddFriend(targetId) {
  const result = await userStore.addFriendAction(targetId)
  if (!result.success) {
    alert(result.message)
  }
}

function handleLogout() {
  chatSocket.disconnect()
  userStore.logout()
  router.push('/login')
}
</script>

<style scoped>
.chat-container {
  display: flex;
  height: 100vh;
  background: #f0f2f5;
}

.chat-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.chat-placeholder {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: #999;
  gap: 16px;
}

.placeholder-icon {
  font-size: 48px;
}

.chat-placeholder p {
  font-size: 16px;
  color: #888;
}

</style>
