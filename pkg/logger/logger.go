package logger

import (
	"os"
	"path/filepath"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Options struct {
	Mode  string
	Level string
	Path  string
	Name  string
}

type Logger interface {
	Debug(args ...interface{})
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})

	Debugf(template string, args ...interface{})
	Infof(template string, args ...interface{})
	Warnf(template string, args ...interface{})
	Errorf(template string, args ...interface{})

	Debugw(msg string, keysAndValues ...interface{})
	Infow(msg string, keysAndValues ...interface{})
	Warnw(msg string, keysAndValues ...interface{})
	Errorw(msg string, keysAndValues ...interface{})
}

var logging Logger

func init() {
	logging = zap.NewNop().Sugar()
}

func NewLogger(o *Options) Logger {
	var (
		logger *zap.Logger
		err    error
	)

	switch o.Mode {
	case "file":
		var level zapcore.Level
		level, err = zapcore.ParseLevel(o.Level)
		if err != nil {
			return zap.NewNop().Sugar()
		}
		if o.Path == "" {
			o.Path = "./logs"
		}
		if o.Name == "" {
			path, _ := os.Executable()
			_, exec := filepath.Split(path)
			o.Name = exec
		}
		fileName := filepath.Join(o.Path, o.Name+".log")
		syncWriter := zapcore.AddSync(&lumberjack.Logger{
			Filename:   fileName,
			MaxSize:    100,
			MaxBackups: 10,
			LocalTime:  true,
			Compress:   true,
		})
		encoder := zap.NewProductionEncoderConfig()
		encoder.EncodeTime = zapcore.ISO8601TimeEncoder
		core := zapcore.NewCore(zapcore.NewJSONEncoder(encoder), syncWriter, zap.NewAtomicLevelAt(level))
		logger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	default:
		config := zap.NewDevelopmentConfig()
		config.DisableStacktrace = true
		//config.OutputPaths = []string{"stdout"}
		logger, err = config.Build(zap.AddCallerSkip(1))
		if err != nil {
			return zap.NewNop().Sugar()
		}
	}
	return logger.Sugar()
}

func SetLogger(logger Logger) {
	logging = logger
}

func Debug(args ...interface{}) {
	logging.Debug(args...)
}
func Debugf(msg string, args ...interface{}) {
	logging.Debugf(msg, args...)
}
func Debugw(msg string, keysAndValues ...interface{}) {
	logging.Debugw(msg, keysAndValues...)
}

func Info(args ...interface{}) {
	logging.Info(args...)
}
func Infof(msg string, args ...interface{}) {
	logging.Infof(msg, args...)
}
func Infow(msg string, keysAndValues ...interface{}) {
	logging.Infow(msg, keysAndValues...)
}

func Warn(args ...interface{}) {
	logging.Warn(args...)
}
func Warnf(msg string, args ...interface{}) {
	logging.Warnf(msg, args...)
}
func Warnw(msg string, keysAndValues ...interface{}) {
	logging.Warnw(msg, keysAndValues...)
}

func Error(args ...interface{}) {
	logging.Error(args...)
}
func Errorf(msg string, args ...interface{}) {
	logging.Errorf(msg, args...)
}
func Errorw(msg string, keysAndValues ...interface{}) {
	logging.Errorw(msg, keysAndValues...)
}
