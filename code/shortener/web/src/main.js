import { createApp } from 'vue'
import App from './App.vue'
import './styles.css'

// 整个前端的入口文件：创建 Vue 应用实例，把根组件 App 挂载到 index.html 中的 #app 节点上。
createApp(App).mount('#app')
