import request from './request'

// 用户注册
export function registerUser(name, password, repassword) {
  return request.get('/user/CreateUser', {
    params: { name, password, repassword }
  })
}

// 用户登录
export function loginUser(name, password) {
  return request.post('/user/FindUserByNameAndPassword', null, {
    params: { name, password }
  })
}

// 获取用户列表
export function getUserList() {
  return request.get('/user/GetUserList')
}

// 删除用户
export function deleteUser(id) {
  return request.get('/user/DeleteUser', { params: { id } })
}

// 更新用户信息
export function updateUser(data) {
  const formData = new URLSearchParams()
  formData.append('id', data.id)
  if (data.name) formData.append('name', data.name)
  if (data.password) formData.append('password', data.password)
  if (data.email) formData.append('email', data.email)
  if (data.phone) formData.append('phone', data.phone)

  return request.post('/user/UpdateUser', formData, {
    headers: {
      'Content-Type': 'application/x-www-form-urlencoded'
    }
  })
}

// 获取好友列表
export function getFriendList(userid) {
  return request.get('/user/SearchFriend', {
    params: { userid }
  })
}

// 添加好友
export function addFriend(userId, targetId) {
  const formData = new URLSearchParams()
  formData.append('userId', userId)
  formData.append('targetId', targetId)
  return request.post('/user/AddFriend', formData, {
    headers: {
      'Content-Type': 'application/x-www-form-urlencoded'
    }
  })
}

// 上传图片
export function uploadImage(file) {
  const formData = new FormData()
  formData.append('image', file)
  return request.post('/user/UploadImage', formData, {
    headers: {
      'Content-Type': 'multipart/form-data'
    },
    timeout: 30000
  })
}

// 获取聊天记录（分页）
export function getChatRecord(userId, targetId, page = 1, pageSize = 20) {
  return request.get('/user/GetChatRecord', {
    params: { userId, targetId, page, pageSize }
  })
}

// 发送消息（HTTP POST，可靠通道）
export function sendMessageHttp(data) {
  const formData = new URLSearchParams()
  formData.append('FormId', data.FormId)
  formData.append('targetId', data.targetId)
  formData.append('content', data.content)
  formData.append('type', data.type || 1)
  formData.append('media', data.media || '1')
  formData.append('pic', data.pic || '')
  return request.post('/user/SendMessageHttp', formData, {
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' }
  })
}

// 上传语音
export function uploadVoice(file) {
  const formData = new FormData()
  formData.append('audio', file)
  return request.post('/user/UploadAudio', formData, {
    headers: {
      'Content-Type': 'multipart/form-data'
    },
    timeout: 30000
  })
}