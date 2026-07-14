import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { loginUser, registerUser, getUserList, updateUser as updateUserApi, getFriendList, addFriend as addFriendApi } from '../api/user'

export const useUserStore = defineStore('user', () => {
  // 当前用户信息
  const currentUser = ref(loadUserFromStorage())
  // 用户列表（聊天室成员）
  const userList = ref([])
  // 好友列表
  const friendList = ref([])
  // 在线用户 ID 集合
  const onlineUserIds = ref(new Set())

  const isLoggedIn = computed(() => !!currentUser.value?.ID)

  function normalizeId(value) {
    if (value === undefined || value === null || value === '') return null
    const numericValue = Number(value)
    return Number.isNaN(numericValue) ? value : numericValue
  }

  function normalizeUser(user) {
    if (!user || typeof user !== 'object') return null

    const rawId = user.ID ?? user.id ?? user.UserId ?? user.userId ?? user.user_id ?? user.UserID ?? user.userID
    const rawName = user.Name ?? user.name ?? user.UserName ?? user.userName ?? user.username ?? ''
    const rawIdentity = user.Identity ?? user.identity ?? user.Token ?? user.token ?? ''
    const normalizedId = normalizeId(rawId)
    const normalizedName = rawName === undefined || rawName === null ? '' : String(rawName)
    const normalizedIdentity = rawIdentity === undefined || rawIdentity === null ? '' : String(rawIdentity)

    if (normalizedId === null && !normalizedName && !normalizedIdentity) {
      return null
    }

    return {
      ...user,
      ID: normalizedId,
      id: normalizedId,
      Name: normalizedName,
      Identity: normalizedIdentity,
      identity: normalizedIdentity
    }
  }

  // 将 API 返回的用户/好友/联系人对象统一规范化为 { ID, Name, ... }
  // 兼容后端可能返回的各种字段名（PascalCase, camelCase, snake_case）
  function normalizeContact(contact) {
    if (!contact || typeof contact !== 'object') return null
    const id = normalizeId(
      contact.ID ?? contact.id ?? contact.UserId ?? contact.userId ??
      contact.FriendId ?? contact.friendId ?? contact.FriendID ?? contact.friendID ??
      contact.user_id ?? contact.friend_id
    )
    const name = contact.Name ?? contact.name ?? contact.UserName ?? contact.userName ??
      contact.FriendName ?? contact.friendName ?? contact.username ??
      contact.Nickname ?? contact.nickname ?? ''
    const phone = contact.Phone ?? contact.phone ?? contact.Mobile ?? contact.mobile ?? ''
    const email = contact.Email ?? contact.email ?? ''

    if (id === null && !name) return null

    return {
      ...contact,
      ID: id,
      id: id,
      Name: name || '',
      name: name || '',
      Phone: phone,
      phone: phone,
      Email: email,
      email: email,
      // 保留原始字段以防后续需要
      UserId: contact.UserId ?? contact.userId ?? id,
      UserName: contact.UserName ?? contact.userName ?? name,
      FriendId: contact.FriendId ?? contact.friendId ?? id,
      FriendName: contact.FriendName ?? contact.friendName ?? name
    }
  }

  function getStoredIdentity() {
    try {
      // sessionStorage 优先，保证同标签页用户不变
      return sessionStorage.getItem('ginchat_user_identity') || localStorage.getItem('ginchat_user_identity') || localStorage.getItem('ginchat_identity') || ''
    } catch {
      return ''
    }
  }

  function getLastLoginIdentity() {
    try {
      // sessionStorage 优先，避免跨标签页用户互相覆盖
      const lastLoginName = sessionStorage.getItem('ginchat_last_login_name') || localStorage.getItem('ginchat_last_login_name') || localStorage.getItem('ginchat_user_name') || ''
      const lastLoginId = sessionStorage.getItem('ginchat_last_login_id') || localStorage.getItem('ginchat_last_login_id') || localStorage.getItem('ginchat_user_id') || ''
      const lastIdentity = sessionStorage.getItem('ginchat_user_identity') || localStorage.getItem('ginchat_user_identity') || localStorage.getItem('ginchat_identity') || ''
      return {
        name: lastLoginName,
        id: lastLoginId,
        identity: lastIdentity
      }
    } catch {
      return { name: '', id: '', identity: '' }
    }
  }

  function persistLoginIdentity(user, loginName = '') {
    const normalizedUser = normalizeUser(user)
    if (!normalizedUser) {
      clearUserFromStorage()
      return null
    }

    const lastIdentity = getLastLoginIdentity()
    const resolvedName = (loginName || normalizedUser.Name || lastIdentity.name || '').toString().trim()
    const resolvedId = normalizeId(normalizedUser.ID ?? normalizedUser.id ?? lastIdentity.id ?? '')
    const resolvedIdentity = normalizedUser.Identity || normalizedUser.identity || lastIdentity.identity || ''

    const finalUser = {
      ...normalizedUser,
      ID: resolvedId,
      id: resolvedId,
      Name: resolvedName || normalizedUser.Name || '',
      Identity: resolvedIdentity,
      identity: resolvedIdentity
    }

    const userPayload = JSON.stringify(finalUser)
    // 同时写入两个存储，sessionStorage 优先读取
    sessionStorage.setItem('ginchat_user', userPayload)
    localStorage.setItem('ginchat_user', userPayload)

    if (resolvedName) {
      sessionStorage.setItem('ginchat_last_login_name', resolvedName)
      localStorage.setItem('ginchat_last_login_name', resolvedName)
      localStorage.setItem('ginchat_user_name', resolvedName)
    }
    if (resolvedId !== null && resolvedId !== '' && resolvedId !== undefined) {
      sessionStorage.setItem('ginchat_last_login_id', String(resolvedId))
      localStorage.setItem('ginchat_last_login_id', String(resolvedId))
      localStorage.setItem('ginchat_user_id', String(resolvedId))
    }
    if (resolvedIdentity) {
      sessionStorage.setItem('ginchat_user_identity', resolvedIdentity)
      localStorage.setItem('ginchat_user_identity', resolvedIdentity)
      localStorage.setItem('ginchat_identity', resolvedIdentity)
    }

    return finalUser
  }

  function loadUserFromStorage() {
    try {
      // sessionStorage 优先：同一标签页的用户身份不会因为其他标签页登录不同用户而改变
      const saved = sessionStorage.getItem('ginchat_user') || localStorage.getItem('ginchat_user')
      if (!saved) return null
      const parsed = JSON.parse(saved)
      // 拒绝无效的存储值（如 "null", "undefined", 空对象等）
      if (!parsed || typeof parsed !== 'object' || (!parsed.ID && !parsed.id)) {
        // 如果 JSON 解析成功但缺少关键字段，尝试从 identity 兜底
        if (!parsed || typeof parsed !== 'object') return null
      }
      const normalizedUser = normalizeUser(parsed)
      if (!normalizedUser) return null

      // ID 以存储的用户数据为准，不沿用 lastIdentity
      // Name 以存储用户数据为准，仅在缺失时补充
      const lastIdentity = getLastLoginIdentity()
      const resolvedId = normalizedUser.ID ?? normalizedUser.id ?? normalizeId(lastIdentity.id || '')
      const resolvedName = (normalizedUser.Name || lastIdentity.name || '').toString().trim()
      const resolvedIdentity = normalizedUser.Identity || normalizedUser.identity || lastIdentity.identity || getStoredIdentity()

      if (resolvedId === null && !resolvedName) {
        return null
      }

      return {
        ...normalizedUser,
        ID: resolvedId,
        id: resolvedId,
        Name: resolvedName || normalizedUser.Name || '',
        Identity: resolvedIdentity,
        identity: resolvedIdentity
      }
    } catch {
      return null
    }
  }

  function saveUserToStorage(user, loginName = '') {
    const normalizedUser = persistLoginIdentity(user, loginName)
    if (!normalizedUser) {
      clearUserFromStorage()
      return
    }
    sessionStorage.setItem('ginchat_user', JSON.stringify(normalizedUser))
    localStorage.setItem('ginchat_user', JSON.stringify(normalizedUser))
  }

  function restoreCurrentUser() {
    // 如果内存中已经有有效用户，直接返回
    if (currentUser.value?.ID) {
      return currentUser.value
    }

    // 从存储中恢复
    const restoredUser = loadUserFromStorage()
    if (restoredUser && restoredUser.ID !== null && restoredUser.ID !== undefined) {
      currentUser.value = restoredUser
    }
    return currentUser.value
  }

  function clearUserFromStorage() {
    localStorage.removeItem('ginchat_user')
    sessionStorage.removeItem('ginchat_user')
    localStorage.removeItem('ginchat_user_name')
    localStorage.removeItem('ginchat_user_id')
    localStorage.removeItem('ginchat_last_login_name')
    localStorage.removeItem('ginchat_last_login_id')
    localStorage.removeItem('ginchat_user_identity')
    localStorage.removeItem('ginchat_identity')
    sessionStorage.removeItem('ginchat_user_identity')
    sessionStorage.removeItem('ginchat_last_login_name')
    sessionStorage.removeItem('ginchat_last_login_id')
  }

  /**
   * 用户注册
   */
  async function register(name, password, repassword) {
    const res = await registerUser(name, password, repassword)
    if (res.code === 0) {
      return { success: true, message: res.message }
    }
    return { success: false, message: res.message || '注册失败' }
  }

  /**
   * 用户登录
   */
  async function login(name, password) {
    const res = await loginUser(name, password)
    const normalizedUser = normalizeUser(res.data)
    if (res.code === 0 && normalizedUser) {
      const finalUser = persistLoginIdentity(normalizedUser, name)
      currentUser.value = finalUser
      return { success: true, message: res.message }
    }
    return { success: false, message: res.message || '登录失败' }
  }

  /**
   * 退出登录
   */
  function logout() {
    currentUser.value = null
    clearUserFromStorage()
  }

  /**
   * 更新用户信息
   */
  async function updateProfile(data) {
    const res = await updateUserApi(data)
    if (res.code === 0) {
      // 更新本地存储
      if (currentUser.value) {
        const mergedUser = normalizeUser({ ...currentUser.value, ...data })
        const finalUser = persistLoginIdentity(mergedUser, currentUser.value?.Name)
        currentUser.value = finalUser
      }
      return { success: true, message: res.message }
    }
    return { success: false, message: res.message || '更新失败' }
  }

  /**
   * 获取用户列表
   */
  async function fetchUserList() {
    const res = await getUserList()
    if (res.code === 0 && res.data) {
      userList.value = (res.data || []).map(normalizeContact).filter(Boolean)
      return userList.value
    }
    return []
  }

  /**
   * 获取好友列表
   */
  async function fetchFriendList() {
    if (!currentUser.value?.ID) return []
    const res = await getFriendList(currentUser.value.ID)
    if (res.code === 0) {
      // 兼容迁移期两种响应格式：data (common.Success) 或 rows (RespList)
      const raw = res.data !== undefined ? res.data : res.rows
      friendList.value = (raw || []).map(normalizeContact).filter(Boolean)
      return friendList.value
    }
    return []
  }

  /**
   * 添加好友
   */
  async function addFriendAction(targetId) {
    if (!currentUser.value?.ID) {
      return { success: false, message: '未登录' }
    }
    const res = await addFriendApi(currentUser.value.ID, targetId)
    if (res.code === 0) {
      await fetchFriendList()
      return { success: true, message: res.message }
    }
    return { success: false, message: res.message || '添加好友失败' }
  }

  /**
   * 判断是否是好友
   */
  function isFriend(userId) {
    return friendList.value.some(f => f.ID === userId)
  }

  /**
   * 设置用户在线状态
   */
  function setUserOnline(userId, online = true) {
    if (online) {
      onlineUserIds.value.add(userId)
    } else {
      onlineUserIds.value.delete(userId)
    }
  }

  return {
    currentUser,
    userList,
    friendList,
    onlineUserIds,
    isLoggedIn,
    login,
    register,
    logout,
    restoreCurrentUser,
    updateProfile,
    fetchUserList,
    fetchFriendList,
    addFriendAction,
    isFriend,
    setUserOnline,
    normalizeContact
  }
})
