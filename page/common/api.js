export const getBase = () => uni.getStorageSync('api_base') || 'http://127.0.0.1:8080'
export const setBase = value => {
 const url = value.trim().replace(/\/+$/, '')
 if (!/^https?:\/\/[^\s/]+(?::\d+)?$/.test(url)) throw new Error('请输入完整服务地址，例如 https://study.example.com')
 uni.setStorageSync('api_base', url)
}
export function request(path, method = 'GET', data) {
 return new Promise((resolve, reject) => uni.request({
  url: getBase() + '/api' + path, method, data, timeout: 12000,
  header: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + (uni.getStorageSync('token') || '') },
  success: res => {
   if (res.statusCode >= 200 && res.statusCode < 300) return resolve(res.data)
   if (res.statusCode === 401 && path !== '/login' && path !== '/register') {
    uni.removeStorageSync('token'); uni.$emit('session-expired')
   }
   reject(new Error((res.data && res.data.error) || '请求失败，请稍后重试'))
  },
  fail: () => reject(new Error('无法连接服务器，请检查服务地址与网络'))
 }))
}
