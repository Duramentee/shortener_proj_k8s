import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Vite 配置说明：
//   1. 开发模式（npm run dev）下，Vite 自己启动一个开发服务器，此时 nginx 不参与工作，
//      因此下面的 server.proxy 把后端请求转发到宿主机上运行的后端（默认端口 8080），
//      目的是让开发模式与「由 nginx 反向代理」的部署模式使用同一套请求路径，前端代码不需要做任何区分。
//   2. server.proxy 的键有两种写法。写成普通字符串时按请求路径的前缀匹配；
//      写成以 ^ 开头的字符串时，Vite 会把这个键解释为正则表达式，
//      依据是 Vite 官方 server.proxy 选项的说明：「If the key starts with ^, it will be interpreted as a RegExp」。
//      下面第二条规则使用正则，作用与 nginx.conf 中的 location ~ "^/[A-Za-z0-9]{6}$" 相同，
//      因此开发模式下也可以直接访问 http://localhost:5173/a1B2c3 来验证 302 跳转。
//   3. 正则规则 ^/[A-Za-z0-9]{6}$ 也能匹配 /assets 这六个字母，
//      但是开发模式下 Vite 不通过这个路径提供文件（开发模式下模块路径形如 /src/main.js 与 /@vite/client），
//      所以这条规则在开发模式下不会误伤静态资源。
//   4. 构建产物输出到 dist/ 目录，Dockerfile 会把这个目录复制进 nginx 镜像的静态文件根目录。
export default defineConfig({
  plugins: [vue()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '^/[A-Za-z0-9]{6}$': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    sourcemap: false
  }
})
