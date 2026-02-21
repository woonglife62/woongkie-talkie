package logger

import "go.uber.org/zap"

// AuditLog records a security-relevant event with structured fields.
// event: short identifier (e.g. "login_success", "room_created")
// actor: username or IP performing the action
// fields: additional key-value context pairs
func AuditLog(event, actor string, fields ...zap.Field) {
	if RawLogger == nil {
		return
	}
	base := []zap.Field{
		zap.String("audit_event", event),
		zap.String("actor", actor),
	}
	RawLogger.With(base...).Info("AUDIT", fields...)
}
