# internal/config/config_test.go

覆盖 PM_TOS_ENABLED/ACCESS_KEY/SECRET_KEY/BUCKET/PRESIGN_TTL/MAX_UPLOAD_BYTES 环境变量以及 key_prefix 默认值，验证缺失 tos 配置段仍可环境覆盖，避免 Viper 未注册 key 静默忽略。
