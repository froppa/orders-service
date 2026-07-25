package provenance

import "go.uber.org/zap"

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func Fields() []zap.Field {
	return []zap.Field{
		zap.String("version", Version),
		zap.String("commit", Commit),
		zap.String("build_date", Date),
	}
}
