package httpapi

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

// loggingMiddleware 记录每一次请求的方法、路径、状态码与耗时。
//
// 本函数已经写好，你不需要修改它。
//
// gin 的中间件就是一个签名为 func(c *gin.Context) 的函数。
// c.Next() 表示继续执行后面的中间件与处理函数，并且在这一行返回时全部处理都已经完成，
// 因此状态码与耗时的读取必须写在 c.Next() 之后。
func (s *Server) loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		s.logger.Info("请求处理完成",
			"方法", c.Request.Method,
			"路径", c.Request.URL.Path,
			"状态码", c.Writer.Status(),
			"耗时", time.Since(start).String(),
			"客户端地址", c.ClientIP(),
		)
	}
}

// recoveryMiddleware 捕获处理函数中的 panic，并且把它转换成 500 响应。
//
// 本函数已经写好，你不需要修改它。以下两点需要理解清楚。
//
// 第一点：为什么必须在收尾时调用 c.Abort()。
// gin 的中间件链用一个下标表示当前执行到第几个处理函数，c.Next() 只是在同一个循环里
// 递增这个下标并且依次调用。当某个处理函数发生 panic 时，panic 会沿着 c.Next() 的调用栈向外传播，
// 直到被某个中间件的 defer 捕获。捕获之后这个中间件正常返回给调用方，
// 而调用方（也就是外层中间件里的那个 c.Next() 循环）会继续递增下标并继续执行后续的处理函数，
// 结果就是同一个处理函数被第二次执行。调用 c.Abort() 会把下标直接设置为终止值，
// 外层的循环因此不再继续。
//
// 第二点：为什么要判断响应是否已经写出。
// 处理函数可能已经写出一部分响应之后才发生 panic，此时状态码无法再修改，
// 强行写出 500 只会产生一条「响应头已经写出」的警告，并且把响应体拼成非法的 JSON。
// 这种情况下只终止后续的处理函数，把状态码保持为已经写出的那个。
func (s *Server) recoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			s.logger.Error("处理请求时发生 panic",
				"方法", c.Request.Method,
				"路径", c.Request.URL.Path,
				"panic", fmt.Sprint(recovered),
				"调用栈", string(debug.Stack()),
			)

			if c.Writer.Written() {
				c.Abort()
				return
			}

			c.AbortWithStatusJSON(http.StatusInternalServerError, errorResponse{Error: "服务器内部错误"})
		}()

		c.Next()
	}
}
