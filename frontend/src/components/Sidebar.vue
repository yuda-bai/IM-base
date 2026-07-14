<template>
  <aside class="sidebar">
    <!-- 当前用户信息 -->
    <div class="sidebar-header">
      <div class="current-user">
        <el-avatar :size="44" :style="{ background: 'linear-gradient(135deg, #667eea, #764ba2)' }">
          {{ currentUser?.Name?.charAt(0)?.toUpperCase() || '?' }}
        </el-avatar>
        <div class="user-info">
          <span class="user-name">{{ currentUser?.Name || '未登录' }}</span>
          <span class="user-status">🟢 在线</span>
        </div>
      </div>
      <div class="header-actions">
        <el-button :icon="Edit" circle @click="$emit('edit-profile')" title="编辑资料" />
        <el-button :icon="SwitchButton" circle @click="$emit('logout')" title="退出登录" />
      </div>
    </div>

    <!-- Tab 切换栏 -->
    <el-tabs v-model="activeTab" class="sidebar-tabs" stretch>
      <el-tab-pane name="chat">
        <template #label>
          <span>💬 聊天室</span>
        </template>
      </el-tab-pane>
      <el-tab-pane name="friends">
        <template #label>
          <span>👥 好友</span>
        </template>
      </el-tab-pane>
    </el-tabs>

    <!-- 搜索框 -->
    <div class="search-box">
      <el-input
        v-model="searchQuery"
        :placeholder="activeTab === 'chat' ? '搜索用户...' : '搜索好友...'"
        :prefix-icon="Search"
        clearable
      />
    </div>

    <!-- ==================== 聊天室 Tab ==================== -->
    <template v-if="activeTab === 'chat'">
      <div class="section-title">
        <span>聊天室成员</span>
        <span class="member-count">{{ onlineCount }}/{{ filteredUsers.length }}</span>
      </div>

      <div class="user-list">
        <div
          v-for="user in filteredUsers"
          :key="user.ID"
          class="user-item"
          :class="{ online: onlineIds.has(user.ID) }"
        >
          <el-badge is-dot :hidden="!onlineIds.has(user.ID)" :offset="[-4, 4]">
            <el-avatar :size="40" :style="{ background: '#5865f2' }">
              {{ user.Name?.charAt(0)?.toUpperCase() || '?' }}
            </el-avatar>
          </el-badge>
          <div class="user-detail">
            <span class="user-name">{{ user.Name }}</span>
            <span class="user-subtitle">
              {{ user.Phone || user.Email || '暂无简介' }}
            </span>
          </div>
        </div>

        <div v-if="filteredUsers.length === 0" class="empty-list">
          暂无成员
        </div>
      </div>

      <div class="sidebar-footer">
        <el-button :icon="Refresh" @click="$emit('refresh-users')" style="width: 100%">
          刷新列表
        </el-button>
      </div>
    </template>

    <!-- ==================== 好友 Tab ==================== -->
    <template v-if="activeTab === 'friends'">
      <div class="section-title">
        <span>好友列表</span>
        <span class="member-count">{{ onlineFriendCount }}/{{ filteredFriends.length }}</span>
      </div>

      <div class="user-list">
        <div
          v-for="friend in filteredFriends"
          :key="friend.ID"
          class="user-item"
          :class="{ online: onlineIds.has(friend.ID), selected: activeFriendId === friend.ID }"
          @click="handleSelectFriend(friend)"
        >
          <el-badge is-dot :hidden="!onlineIds.has(friend.ID)" :offset="[-4, 4]">
            <el-avatar :size="40" :style="{ background: '#5865f2' }">
              {{ friend.Name?.charAt(0)?.toUpperCase() || '?' }}
            </el-avatar>
          </el-badge>
          <div class="user-detail">
            <span class="user-name">{{ friend.Name }}</span>
            <span class="user-subtitle">
              {{ friend.Phone || friend.Email || '暂无简介' }}
            </span>
          </div>
          <span class="friend-status">
            {{ onlineIds.has(friend.ID) ? '🟢' : '⚫' }}
          </span>
        </div>

        <div v-if="filteredFriends.length === 0" class="empty-list">
          😕 暂无好友，快去添加吧！
        </div>
      </div>

      <!-- 添加好友面板 -->
      <div class="add-friend-section">
        <el-button
          :icon="showAddFriend ? Minus : Plus"
          @click="showAddFriend = !showAddFriend"
          style="width: 100%"
        >
          {{ showAddFriend ? '收起' : '添加好友' }}
        </el-button>

        <div v-if="showAddFriend" class="add-friend-panel">
          <el-input
            v-model="addFriendQuery"
            placeholder="输入用户名搜索..."
            clearable
          />
          <div v-if="addFriendQuery.trim()" class="search-results">
            <div
              v-for="user in searchResults"
              :key="user.ID"
              class="search-result-item"
            >
              <div class="result-info">
                <el-avatar :size="32" :style="{ background: '#5865f2' }">
                  {{ user.Name?.charAt(0)?.toUpperCase() }}
                </el-avatar>
                <div class="result-detail">
                  <span class="result-name">{{ user.Name }}</span>
                  <span class="result-sub">{{ user.Phone || user.Email || '' }}</span>
                </div>
              </div>
              <el-button :icon="Plus" circle size="small" @click="handleAddFriend(user)" />
            </div>
            <div v-if="searchResults.length === 0" class="no-results">
              未找到可添加的用户
            </div>
          </div>
        </div>
      </div>

      <div class="sidebar-footer">
        <el-button :icon="Refresh" @click="$emit('refresh-friends')" style="width: 100%">
          刷新好友
        </el-button>
      </div>
    </template>
  </aside>
</template>

<script setup>
import { ref, computed } from 'vue'
import { Edit, SwitchButton, Search, Refresh, Plus, Minus } from '@element-plus/icons-vue'

const props = defineProps({
  userList: { type: Array, default: () => [] },
  friendList: { type: Array, default: () => [] },
  onlineIds: { type: Set, default: () => new Set() },
  currentUser: { type: Object, default: null },
  activeFriendId: { type: [Number, String], default: null }
})

const emit = defineEmits(['logout', 'edit-profile', 'refresh-users', 'refresh-friends', 'add-friend', 'select-friend'])

const activeTab = ref('chat')
const searchQuery = ref('')
const showAddFriend = ref(false)
const addFriendQuery = ref('')

const filteredUsers = computed(() => {
  if (!searchQuery.value.trim()) {
    return props.userList
  }
  const q = searchQuery.value.toLowerCase()
  return props.userList.filter(user =>
    user.Name?.toLowerCase().includes(q) ||
    user.Phone?.includes(q) ||
    user.Email?.toLowerCase().includes(q)
  )
})

const onlineCount = computed(() => {
  return props.userList.filter(u => props.onlineIds.has(u.ID)).length
})

const filteredFriends = computed(() => {
  if (!searchQuery.value.trim()) {
    return props.friendList
  }
  const q = searchQuery.value.toLowerCase()
  return props.friendList.filter(f =>
    f.Name?.toLowerCase().includes(q) ||
    f.Phone?.includes(q) ||
    f.Email?.toLowerCase().includes(q)
  )
})

const onlineFriendCount = computed(() => {
  return props.friendList.filter(f => props.onlineIds.has(f.ID)).length
})

const searchResults = computed(() => {
  if (!addFriendQuery.value.trim()) return []
  const q = addFriendQuery.value.toLowerCase()
  return props.userList.filter(u =>
    !props.friendList.some(f => f.ID === u.ID) &&
    u.ID !== props.currentUser?.ID &&
    (u.Name?.toLowerCase().includes(q) ||
     u.Phone?.includes(q) ||
     u.Email?.toLowerCase().includes(q))
  )
})

function handleAddFriend(user) {
  emit('add-friend', user.ID)
  showAddFriend.value = false
  addFriendQuery.value = ''
}

function handleSelectFriend(friend) {
  emit('select-friend', friend)
}
</script>

<style scoped>
.sidebar {
  width: 300px;
  background: #2c2f33;
  color: #fff;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
}

/* ---- 头部 ---- */
.sidebar-header {
  padding: 20px 16px;
  background: #23262a;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.current-user {
  display: flex;
  align-items: center;
  gap: 12px;
}

.user-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.user-name {
  font-size: 16px;
  font-weight: 500;
}

.user-status {
  font-size: 12px;
  color: #43b581;
}

.header-actions {
  display: flex;
  gap: 4px;
}

/* ---- el-tabs 暗色覆写 ---- */
.sidebar :deep(.el-tabs__header) {
  margin-bottom: 0;
  padding: 0 8px;
}

.sidebar :deep(.el-tabs__nav-wrap::after) {
  background-color: #3a3d42;
}

.sidebar :deep(.el-tabs__item) {
  color: #72767d;
  height: 40px;
  line-height: 40px;
  font-size: 14px;
}

.sidebar :deep(.el-tabs__item:hover) {
  color: #b9bbbe;
}

.sidebar :deep(.el-tabs__item.is-active) {
  color: #fff;
}

.sidebar :deep(.el-tabs__active-bar) {
  background-color: #667eea;
}

/* ---- el-input 暗色覆写 ---- */
.sidebar :deep(.el-input__wrapper) {
  background: #40444b;
  box-shadow: none !important;
  border: none;
  border-radius: 6px;
}

.sidebar :deep(.el-input__wrapper:hover) {
  background: #454950;
}

.sidebar :deep(.el-input__inner) {
  color: #fff;
}

.sidebar :deep(.el-input__inner::placeholder) {
  color: #72767d;
}

.sidebar :deep(.el-input .el-input__clear) {
  color: #72767d;
}

/* ---- el-button 暗色覆写 ---- */
.sidebar :deep(.el-button.is-circle) {
  color: #b9bbbe;
  background: transparent;
  border: none;
}

.sidebar :deep(.el-button.is-circle:hover) {
  background: rgba(255, 255, 255, 0.1);
  color: #fff;
}

/* ---- el-badge 暗色覆写 ---- */
.sidebar :deep(.el-badge__content.is-fixed) {
  bottom: calc(14px + var(--el-badge-size) / 2);
  right: calc(10px + var(--el-badge-size) / 2);
}

/* ---- el-badge dot 颜色覆盖 ---- */
.sidebar :deep(.el-badge__content.is-dot) {
  background: #43b581;
}

/* ---- 搜索框 ---- */
.search-box {
  padding: 12px 16px;
}

/* ---- 区域标题 ---- */
.section-title {
  padding: 0 16px 8px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 12px;
  text-transform: uppercase;
  color: #72767d;
  letter-spacing: 0.5px;
}

.member-count {
  font-size: 11px;
  color: #b9bbbe;
}

/* ---- 用户列表 ---- */
.user-list {
  flex: 1;
  overflow-y: auto;
  padding: 0 8px;
}

.user-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.15s;
}

.user-item:hover {
  background: #3a3d42;
}

.user-item.selected {
  background: #5865f2;
}

.user-item.selected .user-name {
  color: #fff;
}

.user-item.online .user-name {
  color: #fff;
}

.user-item:not(.online) .user-name {
  color: #8e9297;
}

.user-detail {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  flex: 1;
}

.user-detail .user-name {
  font-size: 14px;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.user-subtitle {
  font-size: 11px;
  color: #72767d;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.empty-list {
  padding: 32px 16px;
  text-align: center;
  color: #72767d;
  font-size: 14px;
}

/* ---- 好友状态指示 ---- */
.friend-status {
  font-size: 14px;
  flex-shrink: 0;
}

/* ---- 添加好友 ---- */
.add-friend-section {
  padding: 0 16px;
}

.add-friend-panel {
  margin-top: 8px;
}

.search-results {
  max-height: 200px;
  overflow-y: auto;
  margin-top: 8px;
  background: #36393f;
  border-radius: 6px;
}

.search-result-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  transition: background 0.15s;
}

.search-result-item:hover {
  background: #3a3d42;
}

.result-info {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.result-detail {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.result-name {
  font-size: 13px;
  font-weight: 500;
  color: #fff;
}

.result-sub {
  font-size: 11px;
  color: #72767d;
}

.no-results {
  padding: 16px;
  text-align: center;
  color: #72767d;
  font-size: 13px;
}

/* ---- 底部 ---- */
.sidebar-footer {
  padding: 12px 16px;
  border-top: 1px solid #3a3d42;
}
</style>
