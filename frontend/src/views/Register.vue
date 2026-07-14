<template>
  <div class="auth-container">
    <div class="auth-card">
      <div class="auth-header">
        <h1>📝 注册账号</h1>
        <p>加入 GinChat，开始聊天</p>
      </div>

      <el-form @submit.prevent="handleRegister" label-position="top">
        <el-form-item label="用户名">
          <el-input
            v-model="form.name"
            placeholder="请输入用户名"
            :disabled="loading"
          />
        </el-form-item>

        <el-form-item label="密码">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="请输入密码"
            show-password
            :disabled="loading"
          />
        </el-form-item>

        <el-form-item label="确认密码">
          <el-input
            v-model="form.repassword"
            type="password"
            placeholder="请再次输入密码"
            show-password
            :disabled="loading"
          />
        </el-form-item>

        <el-alert
          v-if="errorMsg"
          :title="errorMsg"
          type="error"
          show-icon
          :closable="false"
          style="margin-bottom: 8px"
        />
        <el-alert
          v-if="successMsg"
          :title="successMsg"
          type="success"
          show-icon
          :closable="false"
          style="margin-bottom: 8px"
        />

        <el-form-item>
          <el-button
            type="primary"
            native-type="submit"
            :loading="loading"
            style="width: 100%"
          >
            {{ loading ? '注册中...' : '注 册' }}
          </el-button>
        </el-form-item>
      </el-form>

      <div class="auth-footer">
        已有账号？<router-link to="/login">立即登录</router-link>
      </div>
    </div>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '../stores/user'

const router = useRouter()
const userStore = useUserStore()

const form = reactive({
  name: '',
  password: '',
  repassword: ''
})
const loading = ref(false)
const errorMsg = ref('')
const successMsg = ref('')

async function handleRegister() {
  if (!form.name.trim() || !form.password.trim()) {
    errorMsg.value = '请输入用户名和密码'
    return
  }

  if (form.password !== form.repassword) {
    errorMsg.value = '两次输入的密码不一致'
    return
  }

  if (form.password.length < 6) {
    errorMsg.value = '密码长度不能少于6位'
    return
  }

  loading.value = true
  errorMsg.value = ''
  successMsg.value = ''

  const result = await userStore.register(form.name, form.password, form.repassword)

  loading.value = false

  if (result.success) {
    successMsg.value = result.message + '，3秒后跳转到登录页...'
    setTimeout(() => {
      router.push('/login')
    }, 1500)
  } else {
    errorMsg.value = result.message
  }
}
</script>

<style scoped>
.auth-container {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100vh;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.auth-card {
  width: 400px;
  background: #fff;
  border-radius: 16px;
  padding: 40px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.15);
}

.auth-header {
  text-align: center;
  margin-bottom: 32px;
}

.auth-header h1 {
  font-size: 28px;
  color: #333;
  margin-bottom: 8px;
}

.auth-header p {
  font-size: 14px;
  color: #999;
}

.auth-footer {
  margin-top: 24px;
  text-align: center;
  font-size: 14px;
  color: #999;
}

.auth-footer a {
  color: #667eea;
  text-decoration: none;
  font-weight: 500;
}

.auth-footer a:hover {
  text-decoration: underline;
}
</style>
