COMMENT ON TABLE wechat_merchant_violations IS '微信支付商户处置通知记录；平台收付通与普通服务商 webhook 共用，按微信 record_id 幂等持久化，供平台审计与运营处理';
