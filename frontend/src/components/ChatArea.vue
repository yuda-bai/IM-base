<template>
  <div class="chat-area">
    <!-- 顶部标题栏 -->
    <header class="chat-header">
      <div class="chat-room-info">
        <span class="room-icon">💬</span>
        <div>
          <h2>{{ selectedFriend ? `与 ${selectedFriend.Name || selectedFriend.name || selectedFriend.UserName || selectedFriend.FriendName || '好友'} 私聊` : '聊天室' }}</h2>
          <p>
            <span v-if="connectionStatus === 'connected'" class="status connected">🟢 已连接</span>
            <span v-else-if="connectionStatus === 'connecting'" class="status connecting">🟡 连接中...</span>
            <span v-else class="status disconnected">🔴 已断开</span>
          </p>
        </div>
      </div>
      <el-button
        v-if="connectionStatus === 'disconnected'"
        type="primary"
        size="small"
        @click="$emit('reconnect')"
      >
        重新连接
      </el-button>
    </header>

    <!-- 消息列表 -->
    <div ref="messageList" class="message-list" @scroll="handleScroll">
      <!-- 顶部加载指示器 -->
      <div v-if="isLoadingMore" class="loading-more">
        <span class="loading-dot"></span> 加载中...
      </div>
      <div v-else-if="!hasMore && messages.length > 0" class="no-more">
        — 没有更多消息了 —
      </div>

      <div v-if="messages.length === 0 && !isLoadingMore" class="empty-chat">
        <div class="empty-icon">👋</div>
        <p>欢迎来到聊天室！发送第一条消息吧~</p>
      </div>

      <div
        v-for="(msg, index) in messages"
        :key="msg.messageId || `${msg.timestamp || 'msg'}-${index}`"
        class="message-wrapper"
        :class="{
          'is-self': isMessageSelf(msg),
          'is-system': msg.type === 'system' || msg.type == 3 || msg.type == 0
        }"
      >
        <!-- 系统消息 -->
        <div v-if="msg.type === 'system' || msg.type == 3 || msg.type == 0" class="system-message">
          <span>{{ msg.content }}</span>
        </div>

        <!-- 普通消息 -->
        <template v-else>
          <div class="message-bubble" :class="{ self: isMessageSelf(msg) }">
            <div class="bubble-header" v-if="!isMessageSelf(msg)">
              <span class="sender-name">{{ msg.userName || msg.UserName || msg.user_name || '未知用户' }}</span>
            </div>
            <!-- 图片消息 -->
            <div v-if="isImageMsg(msg)" class="bubble-image">
              <img :src="msg.pic || msg.content" alt="图片" @click="previewImage(msg.pic || msg.content)" />
            </div>
            <!-- 语音消息 -->
            <div v-else-if="isAudioMsg(msg)" class="bubble-audio">
              <button class="audio-play-btn" @click="toggleAudioPlay(msg, $event)" :class="{ playing: playingAudioId === msg.messageId }">
                <span v-if="playingAudioId === msg.messageId">⏸</span>
                <span v-else>▶</span>
              </button>
              <div class="audio-waveform">
                <span v-for="i in 12" :key="i" class="wave-bar" :style="{ animationDelay: `${i * 0.08}s` }"></span>
              </div>
              <span class="audio-duration">{{ formatDuration(msg.duration || 0) }}</span>
            </div>
            <!-- 文本内容 -->
            <div v-if="msg.content && !isImageMsg(msg) && !isAudioMsg(msg)" class="bubble-content">{{ msg.content }}</div>
            <div class="bubble-meta">
              <span v-if="msg.isOffline" class="offline-tag">离线消息</span>
              <span v-else-if="msg.pending" class="pending-tag">发送中</span>
              <span v-else-if="uploading" class="pending-tag">上传中</span>
              <span class="bubble-time">{{ formatTime(msg.timestamp) }}</span>
            </div>
          </div>
        </template>
      </div>
    </div>

    <!-- 输入区域 -->
    <footer class="chat-input-area">
      <!-- 图片预览 -->
      <div v-if="imagePreviewUrl" class="image-preview-bar">
        <div class="preview-thumb">
          <img :src="imagePreviewUrl" alt="预览" />
          <button class="remove-image-btn" @click="removeImage" :disabled="uploading">✖</button>
        </div>
        <span class="preview-label">{{ uploading ? '上传中...' : '图片已选择' }}</span>
      </div>

      <!-- 录音中 -->
      <div v-if="recording" class="recording-bar">
        <div class="recording-indicator">
          <span class="rec-dot"></span>
          <span class="rec-timer">{{ formatDuration(recordingSeconds) }}</span>
        </div>
        <div class="recording-waveform">
          <span v-for="i in 16" :key="i" class="rec-wave-bar" :style="{ animationDelay: `${i * 0.05}s` }"></span>
        </div>
        <button class="rec-cancel-btn" @click="cancelRecording">取消</button>
        <button class="rec-send-btn" @click="stopRecording">发送 ▶</button>
      </div>

      <div class="input-wrapper">
        <textarea
          v-model="inputText"
          class="message-input"
          placeholder="输入消息... (Enter 发送，Shift+Enter 换行)"
          rows="1"
          :disabled="connectionStatus === 'disconnected' || recording"
          @keydown="handleKeydown"
          ref="inputRef"
        ></textarea>
        <div class="input-actions">
          <input
            ref="imageInputRef"
            type="file"
            accept="image/png,image/jpg,image/jpeg,image/gif,image/webp"
            style="display: none"
            @change="handleImageSelected"
          />
          <button
            class="mic-btn"
            type="button"
            :class="{ recording: recording }"
            @click="toggleRecording"
            :disabled="connectionStatus === 'disconnected' || uploading"
            :title="recording ? '停止录音' : '语音消息'"
          >
            🎤
          </button>
          <button class="image-btn" type="button" @click="triggerImagePick" title="发送图片">
            🖼️
          </button>
          <el-popover
            :visible="showEmojiPicker"
            placement="top-end"
            :width="380"
            trigger="click"
            @update:visible="(val) => showEmojiPicker = val"
          >
            <template #reference>
              <button class="emoji-btn" type="button" @click="showEmojiPicker = !showEmojiPicker">😊</button>
            </template>
            <emoji-picker @emoji-click="handleEmojiSelect"></emoji-picker>
          </el-popover>
          <el-button
            type="primary"
            :disabled="(!inputText.trim() && !imageFile) || !selectedFriend"
            @click="sendMessage"
          >
            发送
          </el-button>
        </div>
      </div>
    </footer>

    <!-- 图片全屏预览 -->
    <Teleport to="body">
      <div v-if="previewImageUrl" class="image-overlay" @click="closeImagePreview">
        <img :src="previewImageUrl" alt="图片预览" @click.stop />
        <button class="close-preview-btn" @click="closeImagePreview">✖</button>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import 'emoji-picker-element'
import { ref, watch, nextTick, onUnmounted } from 'vue'

const props = defineProps({
  messages: { type: Array, default: () => [] },
  currentUser: { type: Object, default: null },
  selectedFriend: { type: Object, default: null },
  connectionStatus: { type: String, default: 'connecting' },
  hasMore: { type: Boolean, default: true },
  isLoadingMore: { type: Boolean, default: false }
})

const emit = defineEmits(['send', 'send-image', 'send-voice', 'reconnect', 'load-more'])

const inputText = ref('')
const messageList = ref(null)
const inputRef = ref(null)
const imageInputRef = ref(null)
const showEmojiPicker = ref(false)

// 图片相关
const imageFile = ref(null)
const imagePreviewUrl = ref('')
const uploading = ref(false)
const previewImageUrl = ref('')

// 语音相关
const recording = ref(false)
const recordingSeconds = ref(0)
let mediaRecorder = null
let audioChunks = []
let recordingTimer = null
let recordingStream = null
const playingAudioId = ref(null)
const audioElements = new Map()

const MAX_RECORD_SECONDS = 60

// 自动滚动到底部（监听消息长度变化和最后一条消息的内容变化）
watch(
  () => props.messages.length,
  () => {
    scrollToBottom()
  }
)

// 当消息列表更新（比如最后一条消息从 pending 变为已确认），也滚动
watch(
  () => {
    const msgs = props.messages
    if (msgs.length === 0) return ''
    const last = msgs[msgs.length - 1]
    return last?.content ?? ''
  },
  () => {
    scrollToBottom()
  }
)

function scrollToBottom() {
  nextTick(() => {
    if (messageList.value) {
      messageList.value.scrollTop = messageList.value.scrollHeight
    }
  })
}

function handleScroll() {
  const el = messageList.value
  if (!el || props.isLoadingMore || !props.hasMore) return

  // 滚动到顶部（scrollTop ≤ 50px）时触发加载更多
  if (el.scrollTop <= 50) {
    const prevScrollHeight = el.scrollHeight
    emit('load-more', { prevScrollHeight })
  }
}

function handleKeydown(e) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    if (!props.selectedFriend) {
      alert('请先在左侧选择一个好友')
      return
    }
    if (!inputText.value.trim()) return
    sendMessage()
  }
}

function toggleEmojiPicker() {
  showEmojiPicker.value = !showEmojiPicker.value
}

function insertEmoji(emoji) {
  const textarea = inputRef.value
  if (!emoji) return

  if (!textarea) {
    inputText.value += emoji
    return
  }

  const start = textarea.selectionStart ?? inputText.value.length
  const end = textarea.selectionEnd ?? inputText.value.length
  inputText.value = `${inputText.value.slice(0, start)}${emoji}${inputText.value.slice(end)}`

  nextTick(() => {
    textarea.focus()
    const cursorPosition = start + emoji.length
    textarea.setSelectionRange(cursorPosition, cursorPosition)
  })
}

function handleEmojiSelect(event) {
  const emoji = event?.detail?.unicode || event?.detail?.emoji || ''
  if (emoji) {
    insertEmoji(emoji)
  }
  showEmojiPicker.value = false
}

function normalizeId(id) {
  if (id === undefined || id === null || id === '') return null
  const numericValue = Number(id)
  return Number.isNaN(numericValue) ? id : numericValue
}

function isMessageSelf(msg) {
  if (!msg || !props.currentUser?.ID) return false
  const msgUserId = normalizeId(msg.userId ?? msg.FormId ?? msg.formId ?? msg.UserId)
  const currentId = normalizeId(props.currentUser.ID ?? props.currentUser.id)
  // 两者都为空时无法判断，视为非自己
  if (msgUserId === null || currentId === null) return false
  return msgUserId === currentId
}

function sendMessage() {
  // 有图片待发送 → 发送图片消息
  if (imageFile.value) {
    sendImageMessage()
    return
  }
  // 纯文本消息
  const content = inputText.value.trim()
  if (!content) return

  emit('send', content)
  inputText.value = ''
  nextTick(() => {
    if (inputRef.value) {
      inputRef.value.style.height = 'auto'
    }
  })
}

function triggerImagePick() {
  if (imageInputRef.value) {
    imageInputRef.value.value = ''
    imageInputRef.value.click()
  }
}

function handleImageSelected(event) {
  const file = event.target.files?.[0]
  if (!file) return

  // 校验类型和大小
  const allowedTypes = ['image/png', 'image/jpg', 'image/jpeg', 'image/gif', 'image/webp']
  if (!allowedTypes.includes(file.type)) {
    alert('仅支持 PNG、JPG、JPEG、GIF、WebP 格式的图片')
    return
  }
  if (file.size > 5 * 1024 * 1024) {
    alert('图片大小不能超过 5MB')
    return
  }

  imageFile.value = file
  imagePreviewUrl.value = URL.createObjectURL(file)
  uploading.value = false
}

function removeImage() {
  if (imagePreviewUrl.value) {
    URL.revokeObjectURL(imagePreviewUrl.value)
  }
  imageFile.value = null
  imagePreviewUrl.value = ''
  uploading.value = false
  if (imageInputRef.value) {
    imageInputRef.value.value = ''
  }
}

function sendImageMessage() {
  if (!imageFile.value) return

  // 通知父组件上传并发送图片
  emit('send-image', imageFile.value, {
    onStart: () => {
      uploading.value = true
    },
    onDone: () => {
      uploading.value = false
      removeImage()
    },
    onError: (err) => {
      uploading.value = false
      alert(err || '图片上传失败')
    }
  })
}

function isImageMsg(msg) {
  if (!msg) return false
  return String(msg.media) === '3' || !!(msg.pic)
}

function isAudioMsg(msg) {
  if (!msg) return false
  return String(msg.media) === '4' || !!(msg.audioUrl || msg.audio)
}

function formatDuration(seconds) {
  if (!seconds || seconds <= 0) return '0:00'
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${m}:${s.toString().padStart(2, '0')}`
}

// ==================== 语音录制 ====================

async function toggleRecording() {
  if (recording.value) {
    stopRecording()
  } else {
    startRecording()
  }
}

async function startRecording() {
  if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
    alert('您的浏览器不支持录音功能，请使用 Chrome 或 Edge')
    return
  }

  try {
    recordingStream = await navigator.mediaDevices.getUserMedia({ audio: true })
  } catch (err) {
    console.error('获取麦克风失败:', err)
    alert('无法访问麦克风，请检查权限设置')
    return
  }

  // 优先使用 webm 格式 (Chrome/Firefox)
  let mimeType = 'audio/webm;codecs=opus'
  if (!MediaRecorder.isTypeSupported(mimeType)) {
    mimeType = 'audio/webm'
    if (!MediaRecorder.isTypeSupported(mimeType)) {
      mimeType = 'audio/mp4'
      if (!MediaRecorder.isTypeSupported(mimeType)) {
        mimeType = ''  // 使用默认格式
      }
    }
  }

  const opts = mimeType ? { mimeType } : {}
  mediaRecorder = new MediaRecorder(recordingStream, opts)
  audioChunks = []

  mediaRecorder.ondataavailable = (e) => {
    if (e.data.size > 0) {
      audioChunks.push(e.data)
    }
  }

  mediaRecorder.onstop = () => {
    // 停止所有轨道
    if (recordingStream) {
      recordingStream.getTracks().forEach(t => t.stop())
      recordingStream = null
    }

    if (audioChunks.length === 0) return

    // 从实际录制的 MIME 类型推断扩展名
    const actualMime = mediaRecorder.mimeType || mimeType || 'audio/webm'
    let ext = '.webm'
    if (actualMime.includes('mp4') || actualMime.includes('aac')) ext = '.m4a'
    else if (actualMime.includes('ogg')) ext = '.ogg'
    else if (actualMime.includes('wav')) ext = '.wav'
    else if (actualMime.includes('mp3') || actualMime.includes('mpeg')) ext = '.mp3'

    const audioBlob = new Blob(audioChunks, { type: actualMime || 'audio/webm' })
    const file = new File([audioBlob], `voice_${Date.now()}${ext}`, { type: actualMime || 'audio/webm' })

    // 通知父组件上传并发送语音
    emit('send-voice', file, {
      duration: recordingSeconds.value,
      onStart: () => {
        uploading.value = true
      },
      onDone: () => {
        uploading.value = false
        recording.value = false
        recordingSeconds.value = 0
        clearInterval(recordingTimer)
        recordingTimer = null
      },
      onError: (err) => {
        uploading.value = false
        recording.value = false
        recordingSeconds.value = 0
        clearInterval(recordingTimer)
        recordingTimer = null
        alert(err || '语音发送失败')
      }
    })
  }

  mediaRecorder.start(250) // 每250ms收集一次数据
  recording.value = true
  recordingSeconds.value = 0

  recordingTimer = setInterval(() => {
    recordingSeconds.value++
    if (recordingSeconds.value >= MAX_RECORD_SECONDS) {
      stopRecording()
    }
  }, 1000)
}

function stopRecording() {
  if (mediaRecorder && mediaRecorder.state === 'recording') {
    mediaRecorder.stop()
  }
}

function cancelRecording() {
  clearInterval(recordingTimer)
  recordingTimer = null

  if (mediaRecorder && mediaRecorder.state === 'recording') {
    // onstop 中判断 audioChunks 为空则跳过上传
    mediaRecorder.onstop = () => {
      if (recordingStream) {
        recordingStream.getTracks().forEach(t => t.stop())
        recordingStream = null
      }
    }
    mediaRecorder.stop()
  }

  audioChunks = []
  recording.value = false
  recordingSeconds.value = 0
}

// ==================== 音频播放 ====================

function toggleAudioPlay(msg, event) {
  const msgId = msg.messageId
  const audioUrl = msg.pic || msg.audioUrl || msg.audio || msg.content

  if (!audioUrl) return

  // 如果正在播放同一个，暂停它
  if (playingAudioId.value === msgId) {
    const el = audioElements.get(msgId)
    if (el) {
      el.pause()
      el.currentTime = 0
    }
    playingAudioId.value = null
    return
  }

  // 停止之前播放的音频
  if (playingAudioId.value) {
    const prevEl = audioElements.get(playingAudioId.value)
    if (prevEl) {
      prevEl.pause()
      prevEl.currentTime = 0
    }
  }

  // 创建或复用 Audio 元素
  let audioEl = audioElements.get(msgId)
  if (!audioEl) {
    audioEl = new Audio(audioUrl)
    audioElements.set(msgId, audioEl)
  }

  audioEl.onended = () => {
    playingAudioId.value = null
  }

  audioEl.onerror = () => {
    playingAudioId.value = null
    alert('音频播放失败')
  }

  audioEl.play().catch(err => {
    console.error('音频播放失败:', err)
    playingAudioId.value = null
  })

  playingAudioId.value = msgId
}

function previewImage(url) {
  if (url) {
    previewImageUrl.value = url
  }
}

function closeImagePreview() {
  previewImageUrl.value = ''
}

function formatTime(timestamp) {
  if (!timestamp) return ''
  try {
    const date = new Date(timestamp)
    if (isNaN(date.getTime())) {
      // 尝试从服务器时间戳格式中提取时间部分
      const timeMatch = String(timestamp).match(/(\d{2}:\d{2}(:\d{2})?)/)
      return timeMatch ? timeMatch[1] : ''
    }
    const hours = date.getHours().toString().padStart(2, '0')
    const minutes = date.getMinutes().toString().padStart(2, '0')
    return `${hours}:${minutes}`
  } catch {
    return ''
  }
}

// 暴露方法给父组件
defineExpose({ scrollToBottom, messageList })

// 组件卸载时清理资源
onUnmounted(() => {
  clearInterval(recordingTimer)
  recordingTimer = null
  if (recordingStream) {
    recordingStream.getTracks().forEach(t => t.stop())
    recordingStream = null
  }
  if (mediaRecorder && mediaRecorder.state === 'recording') {
    mediaRecorder.stop()
  }
  // 清理音频元素
  audioElements.forEach(el => {
    el.pause()
    el.src = ''
  })
  audioElements.clear()
  // 清理图片预览 URL
  if (imagePreviewUrl.value) {
    URL.revokeObjectURL(imagePreviewUrl.value)
  }
})
</script>

<style scoped>
.chat-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: #f5f5f5;
}

/* 顶部栏 */
.chat-header {
  height: 60px;
  padding: 0 24px;
  background: #fff;
  border-bottom: 1px solid #e0e0e0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-shrink: 0;
}

.chat-room-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.room-icon {
  font-size: 28px;
}

.chat-room-info h2 {
  font-size: 18px;
  font-weight: 600;
  color: #333;
}

.chat-room-info p {
  font-size: 12px;
  margin-top: 2px;
}

.status.connected { color: #43b581; }
.status.connecting { color: #faa61a; }
.status.disconnected { color: #f04747; }

/* 消息列表 */
.message-list {
  flex: 1;
  overflow-y: auto;
  padding: 20px 24px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.empty-chat {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: #999;
}

.empty-icon {
  font-size: 64px;
  margin-bottom: 16px;
}

.empty-chat p {
  font-size: 16px;
}

/* 加载更多指示器 */
.loading-more,
.no-more {
  text-align: center;
  padding: 12px 0;
  font-size: 13px;
  color: #999;
}

.loading-dot {
  display: inline-block;
  width: 12px;
  height: 12px;
  border: 2px solid #667eea;
  border-top-color: transparent;
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
  vertical-align: middle;
  margin-right: 6px;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* 系统消息 */
.system-message {
  text-align: center;
  padding: 4px 0;
}

.system-message span {
  display: inline-block;
  padding: 4px 16px;
  background: rgba(0, 0, 0, 0.06);
  border-radius: 12px;
  font-size: 12px;
  color: #999;
}

/* 消息气泡 */
.message-wrapper {
  display: flex;
}

.message-wrapper.is-system {
  justify-content: center;
}

.message-wrapper.is-self {
  justify-content: flex-end;
}

.message-bubble {
  max-width: 65%;
  padding: 12px 16px;
  border-radius: 16px;
  background: #fff;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
  position: relative;
}

.message-bubble.self {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
  border-bottom-right-radius: 4px;
}

.message-bubble:not(.self) {
  border-bottom-left-radius: 4px;
}

.bubble-header {
  margin-bottom: 4px;
}

.sender-name {
  font-size: 12px;
  font-weight: 600;
  color: #5865f2;
}

.message-bubble.self .sender-name {
  color: rgba(255, 255, 255, 0.8);
}

.bubble-content {
  font-size: 15px;
  line-height: 1.5;
  word-break: break-word;
}

.bubble-meta {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  margin-top: 4px;
}

.offline-tag,
.pending-tag {
  font-size: 10px;
  padding: 2px 6px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.2);
  color: inherit;
}

.bubble-time {
  font-size: 11px;
  opacity: 0.6;
  text-align: right;
}

/* 输入区域 */
.chat-input-area {
  padding: 16px 24px;
  background: #fff;
  border-top: 1px solid #e0e0e0;
  flex-shrink: 0;
}

.input-wrapper {
  display: flex;
  gap: 12px;
  align-items: flex-end;
  position: relative;
}

.input-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.image-btn,
.emoji-btn {
  width: 40px;
  height: 40px;
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  background: #fff;
  cursor: pointer;
  font-size: 20px;
}

.image-btn:hover,
.emoji-btn:hover {
  background: #f7f7f7;
}

.message-input {
  flex: 1;
  min-height: 40px;
  max-height: 120px;
  padding: 10px 16px;
  border: 1px solid #e0e0e0;
  border-radius: 20px;
  font-size: 14px;
  line-height: 1.5;
  outline: none;
  resize: none;
  font-family: inherit;
  transition: border-color 0.2s;
}

.message-input:focus {
  border-color: #667eea;
}

.message-input:disabled {
  background: #f5f5f5;
}

/* 图片预览条 */
.image-preview-bar {
  padding: 0 24px 10px;
  display: flex;
  align-items: center;
  gap: 10px;
}

.preview-thumb {
  position: relative;
  width: 64px;
  height: 64px;
  border-radius: 8px;
  overflow: hidden;
  border: 2px solid #e0e0e0;
  flex-shrink: 0;
}

.preview-thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.remove-image-btn {
  position: absolute;
  top: 2px;
  right: 2px;
  width: 20px;
  height: 20px;
  border: none;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.6);
  color: #fff;
  font-size: 10px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
}

.remove-image-btn:hover {
  background: rgba(0, 0, 0, 0.85);
}

.preview-label {
  font-size: 13px;
  color: #888;
}

/* 图片消息气泡 */
.bubble-image {
  max-width: 240px;
  border-radius: 8px;
  overflow: hidden;
  cursor: pointer;
}

.bubble-image img {
  width: 100%;
  display: block;
  transition: transform 0.2s;
}

.bubble-image img:hover {
  transform: scale(1.02);
}

/* 全屏图片预览 */
.image-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.85);
  z-index: 9999;
  display: flex;
  align-items: center;
  justify-content: center;
}

.image-overlay img {
  max-width: 90vw;
  max-height: 90vh;
  border-radius: 8px;
  object-fit: contain;
}

.close-preview-btn {
  position: absolute;
  top: 20px;
  right: 20px;
  width: 40px;
  height: 40px;
  border: none;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.2);
  color: #fff;
  font-size: 18px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.2s;
}

.close-preview-btn:hover {
  background: rgba(255, 255, 255, 0.35);
}

/* ==================== 录音条 ==================== */
.recording-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 24px;
  background: #fff5f5;
  border-bottom: 1px solid #ffcccc;
}

.recording-indicator {
  display: flex;
  align-items: center;
  gap: 6px;
}

.rec-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: #f04747;
  animation: rec-pulse 1s infinite;
}

@keyframes rec-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.4; transform: scale(0.7); }
}

.rec-timer {
  font-size: 14px;
  font-weight: 600;
  color: #f04747;
  min-width: 40px;
}

.recording-waveform {
  display: flex;
  align-items: center;
  gap: 2px;
  flex: 1;
  height: 36px;
}

.rec-wave-bar {
  width: 3px;
  background: #f04747;
  border-radius: 2px;
  animation: rec-wave 0.8s ease-in-out infinite alternate;
  flex: 1;
}

@keyframes rec-wave {
  0% { height: 8px; }
  100% { height: 32px; }
}

.rec-cancel-btn {
  height: 32px;
  padding: 0 14px;
  border: 1px solid #ccc;
  border-radius: 16px;
  background: #fff;
  color: #666;
  font-size: 13px;
  cursor: pointer;
  flex-shrink: 0;
}

.rec-cancel-btn:hover {
  background: #f5f5f5;
  border-color: #f04747;
  color: #f04747;
}

.rec-send-btn {
  height: 32px;
  padding: 0 16px;
  border: none;
  border-radius: 16px;
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  flex-shrink: 0;
  transition: opacity 0.2s;
}

.rec-send-btn:hover {
  opacity: 0.9;
}

/* ==================== 麦克风按钮 ==================== */
.mic-btn {
  width: 40px;
  height: 40px;
  border: 1px solid #e0e0e0;
  border-radius: 50%;
  background: #fff;
  cursor: pointer;
  font-size: 20px;
  transition: all 0.2s;
}

.mic-btn:hover:not(:disabled) {
  background: #f7f7f7;
  border-color: #667eea;
}

.mic-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.mic-btn.recording {
  background: #f04747;
  border-color: #f04747;
  color: #fff;
  animation: mic-pulse 1.2s infinite;
}

@keyframes mic-pulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(240, 71, 71, 0.4); }
  70% { box-shadow: 0 0 0 10px rgba(240, 71, 71, 0); }
}

/* ==================== 语音消息气泡 ==================== */
.bubble-audio {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 160px;
  padding: 4px 0;
}

.audio-play-btn {
  width: 36px;
  height: 36px;
  border: none;
  border-radius: 50%;
  background: rgba(102, 126, 234, 0.15);
  color: #667eea;
  font-size: 14px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: background 0.2s;
}

.message-bubble.self .audio-play-btn {
  background: rgba(255, 255, 255, 0.25);
  color: #fff;
}

.audio-play-btn:hover {
  background: rgba(102, 126, 234, 0.3);
}

.audio-play-btn.playing {
  background: #667eea;
  color: #fff;
}

.message-bubble.self .audio-play-btn.playing {
  background: rgba(255, 255, 255, 0.4);
}

.audio-waveform {
  display: flex;
  align-items: center;
  gap: 2px;
  flex: 1;
  height: 28px;
}

.wave-bar {
  width: 2px;
  background: #667eea;
  border-radius: 1px;
  animation: wave-jump 0.7s ease-in-out infinite alternate;
  flex: 1;
}

.message-bubble.self .wave-bar {
  background: rgba(255, 255, 255, 0.8);
}

@keyframes wave-jump {
  0% { height: 6px; }
  100% { height: 22px; }
}

.audio-duration {
  font-size: 12px;
  color: #999;
  flex-shrink: 0;
}

.message-bubble.self .audio-duration {
  color: rgba(255, 255, 255, 0.7);
}
</style>
