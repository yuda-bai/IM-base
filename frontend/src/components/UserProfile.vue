<template>
  <el-dialog
    v-model="visible"
    title="编辑个人资料"
    width="420px"
    :close-on-click-modal="false"
    @closed="$emit('close')"
  >
    <el-form label-position="top">
      <el-form-item label="用户名">
        <el-input
          v-model="form.name"
          placeholder="请输入用户名"
          :disabled="saving"
        />
      </el-form-item>

      <el-form-item label="新密码（留空则不修改）">
        <el-input
          v-model="form.password"
          type="password"
          placeholder="留空则不修改密码"
          show-password
          :disabled="saving"
        />
      </el-form-item>

      <el-form-item label="邮箱">
        <el-input
          v-model="form.email"
          placeholder="请输入邮箱"
          :disabled="saving"
        />
      </el-form-item>

      <el-form-item label="手机号">
        <el-input
          v-model="form.phone"
          placeholder="请输入手机号"
          :disabled="saving"
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
    </el-form>

    <template #footer>
      <el-button @click="handleClose" :disabled="saving">取消</el-button>
      <el-button type="primary" :loading="saving" @click="handleSave">
        {{ saving ? '保存中...' : '保存' }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useUserStore } from '../stores/user'

const props = defineProps({
  user: { type: Object, default: () => ({}) }
})

const emit = defineEmits(['close'])

const userStore = useUserStore()
const visible = ref(true)

const form = reactive({
  name: props.user?.Name || '',
  password: '',
  email: props.user?.Email || '',
  phone: props.user?.Phone || ''
})

const saving = ref(false)
const errorMsg = ref('')
const successMsg = ref('')

function handleClose() {
  visible.value = false
}

async function handleSave() {
  errorMsg.value = ''
  successMsg.value = ''

  const data = { id: props.user?.ID }
  if (form.name.trim()) data.name = form.name
  if (form.password.trim()) data.password = form.password
  if (form.email.trim()) data.email = form.email
  if (form.phone.trim()) data.phone = form.phone

  saving.value = true

  const result = await userStore.updateProfile(data)
  if (result.success) {
    successMsg.value = result.message
    setTimeout(() => {
      visible.value = false
    }, 1000)
  } else {
    errorMsg.value = result.message
  }

  saving.value = false
}
</script>
