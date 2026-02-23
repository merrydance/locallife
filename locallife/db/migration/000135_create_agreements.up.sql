-- Create agreements table
CREATE TABLE agreements (
    id BIGSERIAL PRIMARY KEY,
    type VARCHAR(50) NOT NULL, -- e.g., 'MERCHANT_AGREEMENT', 'USER_AGREEMENT', 'CONSUMER_RIGHTS'
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    version VARCHAR(20) NOT NULL,
    published_on DATE NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_agreements_type_version ON agreements (type, version);
CREATE INDEX idx_agreements_type_active ON agreements (type, is_active);

-- Initial Data Insert for "来富网络（宁晋）有限公司" - 乐客来福 (LeKeLaiFu)
-- Version: v1.1.0-legal
-- Description: Professional legal version incorporating risk isolation, price parity, and automated dispute resolution.

-- 1. 商户入驻及数字化服务协议
INSERT INTO agreements (type, title, content, version, published_on) VALUES (
    'MERCHANT_AGREEMENT',
    '乐客来福商户入驻及数字化服务协议',
    '<div class="agreement-content">
        <h1>乐客来福商户入驻及数字化服务协议</h1>
        <p class="publish-date">发布/生效日期：2026年02月23日</p>
        <p><b>【前言】</b>欢迎入驻乐客来福平台。本协议是来富网络（宁晋）有限公司（下称“平台”）与您（下称“商户”）就数字化门店运营及相关服务达成的具有法律效力的约定。</p>
        
        <h2>第一条 平台定位与独立性</h2>
        <p>1.1 <b>技术服务提供者：</b>平台仅为商户提供数字化经营工具（如扫码点餐、外卖交易撮合等），平台并非餐饮买卖合同的当事人，不实际经营餐饮业务。<br>
        1.2 <b>主体责任：</b>商户作为食品安全的第一责任人，应独立承担因食品质量、虚假宣传、价格欺诈等产生的全部法律责任。</p>
        
        <h2>第二条 资质准入与合规</h2>
        <p>2.1 <b>强制证照：</b>商户必须上传真实的《营业执照》及《食品经营许可证》/《食品安全许可证》。<br>
        2.2 <b>真实经营：</b>严禁虚设地址或无实体店入驻。若发现商户提供虚假资料，平台有权立即清退并追究责任。</p>
        
        <h2>第三条 商业条款：低佣金与同质同价</h2>
        <p>3.1 <b>服务费率：</b>平台收取的数字化服务费实行<b>5%封顶</b>政策，无年度服务费、排名费等隐性支出。<br>
        3.2 <b>价格平权：</b>为维护本地市场秩序，商户在此承诺执行<b>“线上线下同质同价”</b>原则，外卖及预订价格不得高于堂食实际售价。</p>
        
        <h2>第四条 流量分配与排名规则</h2>
        <p>4.1 <b>去算法操纵：</b>平台坚持公平排序，默认以<b>“地理位置距离”</b>作为核心展示逻辑。<br>
        4.2 <b>信用权重：</b>系统自动根据销量、复购率（好评度）及合规记录调整权重，助力优质商户，严禁竞价排名。</p>
        
        <h2>第五条 风险界定与食安熔断</h2>
        <p>5.1 <b>风险交割：</b>食品质量及食品安全责任由商户承担至“取餐确认”环节。取餐后至送达前的非食品质量因素（如包装受损、运输污染）由独立承运人（骑手）承担。<br>
        5.2 <b>食安熔断权：</b>若发生疑似食品安全事故，平台基于公共安全考量有权行使<b>单方熔断权</b>，立即关停商户交易入口，直至风险排查完毕。</p>
        
        <h2>第六条 法律管辖</h2>
        <p>6.1 <b>争议解决：</b>本协议的解释、履行及纠纷解决均适用中华人民共和国法律。若协商不成，提交<b>宁晋县人民法院</b>审理。</p>
    </div>',
    'v1.1.0-legal',
    '2026-02-23'
);

-- 2. 用户服务协议
INSERT INTO agreements (type, title, content, version, published_on) VALUES (
    'USER_AGREEMENT',
    '乐客来福用户服务及隐私协议',
    '<div class="agreement-content">
        <h1>乐客来福用户服务协议</h1>
        <p class="publish-date">发布/生效日期：2026年02月23日</p>
        
        <h2>第一条 服务定义的法律实质</h2>
        <p>1.1 乐客来福为您提供本地商户展示及信息交互技术支持。<br>
        1.2 <b>委托代理关系：</b>您在使用外卖服务时，系统自动匹配独立承运人。您授权该承运人作为您的代理人前往餐厅完成取餐、支付及送达任务。</p>
        
        <h2>第二条 隐私保护承诺</h2>
        <p>2.1 我们尊重您的隐私，除地理位置（用于匹配最近餐厅）及联系方式（用于订单确认）外，我们不额外收集您的无关敏感个人信息。</p>
        
        <h2>第三条 消费保障</h2>
        <p>3.1 平台不对餐厅食品的口感、色泽等主观性因素负责，但会对商户的经营资质进行严格法定义务审核。</p>
    </div>',
    'v1.1.0-legal',
    '2026-02-23'
);

-- 3. 消费者权益保障及纠纷处理协议
INSERT INTO agreements (type, title, content, version, published_on) VALUES (
    'CONSUMER_RIGHTS',
    '乐客来福“极速保障”及平台裁决规则',
    '<div class="agreement-content">
        <h1>乐客来福消费者权益保障协议</h1>
        <p class="publish-date">发布/生效日期：2026年02月23日</p>
        
        <h2>一、 平台先行赔付义务</h2>
        <p>1.1 为优化本地商业生态，针对常见消费争议，平台实行<b>“先行垫付”</b>制度。用户申请合规索赔时，由平台先行退款，平台后续向责任方（商户或骑手）追偿。</p>
        
        <h2>二、 自动行为回溯裁决机制</h2>
        <p>2.1 平台基于大数据及行为轨迹记录，对投诉订单实行<b>“自动裁决”</b>。<br>
        2.2 裁决逻辑以系统回溯为准，用户无需承担复杂的举证义务，系统依据时间戳、GPS位置、状态更新及异常预警进行公正判定。</p>
        
        <h2>三、 食品安全保障金</h2>
        <p>3.1 平台建立食安保障体系。一旦确认为食安事故，平台将强制执行商户赔付，并触发区域性预警通知。</p>
    </div>',
    'v1.1.0-legal',
    '2026-02-23'
);

-- 4. 骑手入驻及承运服务协议
INSERT INTO agreements (type, title, content, version, published_on) VALUES (
    'RIDER_AGREEMENT',
    '乐客来福骑手承运及信用保证金协议',
    '<div class="agreement-content">
        <h1>乐客来福骑手承运服务协议</h1>
        <p class="publish-date">发布/生效日期：2026年02月23日</p>
        
        <h2>第一条 民事主体地位</h2>
        <p>1.1 <b>独立代理人：</b>骑手与平台之间系技术信息合作关系，非劳动合同关系或劳务派遣关系。骑手作为独立民事主体，代表消费者行使取餐及承运职权。<br>
        1.2 <b>零抽成机制：</b>平台承诺不从骑手的劳动所得中抽取任何百分比提成，运费归骑手完整享有。</p>
        
        <h2>第二条 职业准入与健康管理</h2>
        <p>2.1 骑手必须确保持有有效期内的<b>《健康证》</b>，并完成实名身份核验。</p>
        
        <h2>第三条 货物检查与押金保障</h2>
        <p>3.1 <b>取餐核验：</b>骑手应在取餐环节核验包装完整性。确认取餐后，路途中的洒漏、丢失、超时等非食品生产原因产生的风险由骑手承担。<br>
        3.2 <b>履约保证金：</b>为保障代取安全，系统在订单承运期间临时冻结骑手账户等价余额，订单交付通过后实时解冻。</p>
    </div>',
    'v1.1.0-legal',
    '2026-02-23'
);

-- 5. 委托代取代理协议
INSERT INTO agreements (type, title, content, version, published_on) VALUES (
    'PICKUP_PROXY_AGREEMENT',
    '外卖委托代取三方服务协议',
    '<div class="agreement-content">
        <h1>委托代取三方协议</h1>
        <p>1. <b>法律性质：</b>本协议规定消费者（委托人）通过平台匹配骑手（受托人）进行代取餐的法律义务。<br>
        2. <b>合意达成：</b>消费者发起支付运费即达成委托要约，骑手抢单即视为承诺。双方之间的承运风险由骑手依据《民法典》运输合同相关章节承担职责。<br>
        3. <b>平台角色：</b>平台仅作为第三方见证方及资金托管方，不介入具体的代理行为，不承担承运担保责任。</p>
    </div>',
    'v1.1.0-legal',
    '2026-02-23'
);

-- 6. 区县运营商区域管理协议
INSERT INTO agreements (type, title, content, version, published_on) VALUES (
    'OPERATOR_AGREEMENT',
    '乐客来福区县运营商管理与授权协议',
    '<div class="agreement-content">
        <h1>区县运营商管理协议</h1>
        <p>1. <b>区域授权：</b>本协议约束运营商在该行政区域内的运营行为，包括但不限于商户实地核验、骑手宣导、异常订单初审。<br>
        2. <b>合规监管：</b>运营商应配合平台维护宁晋县（或对应区县）餐饮数字化秩序。若发生运营不力或违规招商，平台有权撤销区域管理权限。<br>
        3. <b>身份独立：</b>运营商属于平台合作伙伴，不代表平台行使任何具有行政法法律效力的职权。</p>
    </div>',
    'v1.1.0-legal',
    '2026-02-23'
);
