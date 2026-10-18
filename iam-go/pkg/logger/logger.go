package logger

import (
	"os"
	"time"

	"github.com/phuslu/log"
)

// Init 全局 Logger 初始化
// level: trace / debug / info / warn / error
// format: json / console
// output: stdout / stderr / 文件路径
func Init(level, format, output string) {
	lvl := log.ParseLevel(level)
	log.DefaultLogger = log.Logger{
		Level:      lvl,
		TimeFormat: time.RFC3339Nano,
	}

	switch format {
	case "console":
		log.DefaultLogger.Writer = &log.ConsoleWriter{
			ColorOutput: true,
			Writer:      resolveOutput(output),
		}
	default:
		log.DefaultLogger.Writer = &log.IOWriter{Writer: resolveOutput(output)}
	}
}

func resolveOutput(s string) *os.File {
	switch s {
	case "stderr":
		return os.Stderr
	case "stdout", "":
		return os.Stdout
	default:
		f, err := os.OpenFile(s, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			Warn().Str("file", s).Err(err).Msg("open log file failed, fallback to stdout")
			return os.Stdout
		}
		return f
	}
}

// ---- 便捷方法 ----

func Trace() *log.Entry { return log.DefaultLogger.Trace() }
func Debug() *log.Entry { return log.DefaultLogger.Debug() }
func Info() *log.Entry  { return log.DefaultLogger.Info() }
func Warn() *log.Entry  { return log.DefaultLogger.Warn() }
func Error() *log.Entry { return log.DefaultLogger.Error() }
func Fatal() *log.Entry { return log.DefaultLogger.Fatal() }

// SetLevel 动态调整日志级别
func SetLevel(level log.Level) {
	log.DefaultLogger.Level = level
}

// Printf 兼容旧 printf 风格（简单场景）
func Printf(format string, args ...interface{}) {
	log.DefaultLogger.Info().Msgf(format, args...)
}

func Fatalf(format string, args ...interface{}) {
	log.DefaultLogger.Fatal().Msgf(format, args...)
}

func Println(v ...interface{}) {
	log.DefaultLogger.Info().Msgf("%v", v...)
}