-- 电商 AI 生图平台数据库迁移脚本
-- 基于 GeeKAI 框架扩展，不修改现有表结构

-- Task 1.1: 生图任务主表
CREATE TABLE IF NOT EXISTS `geekai_ai_image_tasks` (
  `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `task_no`         VARCHAR(64)     NOT NULL UNIQUE COMMENT '对外暴露的任务号（雪花ID）',
  `user_id`         INT UNSIGNED    NOT NULL COMMENT '关联 geekai_users.id',
  `module`          VARCHAR(32)     NOT NULL COMMENT 'main_image|detail_page|white_bg|clone|ratio_convert|translate',
  `image_type`      VARCHAR(64)     NULL     COMMENT '图片类型，如引流封面、首屏主视觉',
  `platform`        VARCHAR(32)     NULL     COMMENT 'taobao|jd|amazon|tiktok|generic',
  `language`        VARCHAR(16)     NULL     COMMENT 'zh-CN|en-US|ja-JP|ko-KR',
  `ratio`           VARCHAR(16)     NULL     COMMENT '1:1|3:4|4:3|16:9|9:16',
  `input_json`      JSON            NOT NULL COMMENT '完整请求参数快照',
  `prompt_json`     JSON            NULL     COMMENT '渲染后 prompt 快照',
  `status`          VARCHAR(16)     NOT NULL DEFAULT 'pending' COMMENT 'pending|queued|running|succeeded|failed|cancelled',
  `progress`        TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0-100',
  `model`           VARCHAR(64)     NULL     COMMENT 'kolors|flux|hunyuan',
  `credit_cost`     INT UNSIGNED    NULL     COMMENT '实际扣减算力',
  `credit_tx_id`    VARCHAR(64)     NULL     COMMENT 'GeeKAI 算力交易流水 ID',
  `provider`        VARCHAR(32)     NULL     COMMENT 'siliconflow|aliyun',
  `provider_job_id` VARCHAR(128)    NULL     COMMENT '上游任务 ID',
  `error_code`      VARCHAR(64)     NULL,
  `error_message`   VARCHAR(1024)   NULL,
  `started_at`      DATETIME        NULL,
  `finished_at`     DATETIME        NULL,
  `created_at`      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at`      DATETIME        NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_task_no` (`task_no`),
  KEY `idx_user_module_created` (`user_id`, `module`, `created_at`),
  KEY `idx_status_created` (`status`, `created_at`),
  KEY `idx_provider_job` (`provider`, `provider_job_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='电商 AI 生图任务表';

-- Task 1.2: 图片资产表
CREATE TABLE IF NOT EXISTS `geekai_ai_image_assets` (
  `id`           BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
  `asset_no`     VARCHAR(64)      NOT NULL UNIQUE COMMENT '资产唯一编号',
  `task_id`      BIGINT UNSIGNED  NULL     COMMENT '关联任务 ID（上传的参考图为 NULL）',
  `user_id`      INT UNSIGNED     NOT NULL,
  `kind`         VARCHAR(32)      NOT NULL COMMENT 'reference|generated|intermediate|thumbnail',
  `oss_bucket`   VARCHAR(128)     NOT NULL,
  `oss_key`      VARCHAR(512)     NOT NULL,
  `mime_type`    VARCHAR(64)      NOT NULL,
  `width`        INT UNSIGNED     NULL,
  `height`       INT UNSIGNED     NULL,
  `size_bytes`   BIGINT UNSIGNED  NULL,
  `sha256`       CHAR(64)         NULL,
  `metadata_json` JSON            NULL     COMMENT 'OCR坐标、翻译文本、生成seed等',
  `created_at`   DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `deleted_at`   DATETIME         NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_asset_no` (`asset_no`),
  KEY `idx_user_kind_created` (`user_id`, `kind`, `created_at`),
  KEY `idx_task_kind` (`task_id`, `kind`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='电商 AI 图片资产表';

-- Task 1.3: Prompt 模板表
CREATE TABLE IF NOT EXISTS `geekai_ai_prompt_templates` (
  `id`                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `template_key`      VARCHAR(128)    NOT NULL COMMENT '逻辑键，如 main_image.hero.taobao.zh-CN',
  `module`            VARCHAR(32)     NOT NULL,
  `image_type`        VARCHAR(64)     NOT NULL,
  `platform`          VARCHAR(32)     NOT NULL,
  `language`          VARCHAR(16)     NOT NULL,
  `ratio`             VARCHAR(16)     NOT NULL DEFAULT 'any',
  `model`             VARCHAR(64)     NOT NULL DEFAULT 'kolors',
  `system_prompt`     MEDIUMTEXT      NOT NULL,
  `user_template`     MEDIUMTEXT      NOT NULL COMMENT '含 {{product_name}} 等变量',
  `negative_template` MEDIUMTEXT      NULL,
  `params_json`       JSON            NULL     COMMENT 'guidance_scale、steps 等默认参数',
  `version`           INT UNSIGNED    NOT NULL DEFAULT 1,
  `status`            VARCHAR(16)     NOT NULL DEFAULT 'active' COMMENT 'draft|active|archived',
  `created_by`        INT UNSIGNED    NULL,
  `created_at`        DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`        DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_template_version` (`template_key`, `version`),
  KEY `idx_match` (`module`, `image_type`, `platform`, `language`, `ratio`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='电商 AI Prompt 模板表';

-- Task 1.4: 模型积分定价表
CREATE TABLE IF NOT EXISTS `geekai_ai_model_price_config` (
  `id`               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `model`            VARCHAR(64)     NOT NULL UNIQUE COMMENT 'kolors|flux|hunyuan|rembg|translate',
  `module`           VARCHAR(32)     NOT NULL DEFAULT 'all',
  `credit_per_image` INT UNSIGNED    NOT NULL COMMENT '每张图消耗算力',
  `description`      VARCHAR(255)    NULL,
  `status`           VARCHAR(16)     NOT NULL DEFAULT 'active',
  `updated_at`       DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI 模型积分定价配置';

-- 初始定价数据
INSERT IGNORE INTO `geekai_ai_model_price_config` (`model`, `module`, `credit_per_image`, `description`) VALUES
('kolors',    'all',            10, '硅基流动 Kolors 文生图'),
('flux',      'all',            15, '硅基流动 FLUX 文生图'),
('hunyuan',   'all',            12, '腾讯混元图像生成'),
('rembg',     'white_bg',        5, '背景移除（白底图）'),
('translate', 'translate',       8, '图文翻译');

INSERT IGNORE INTO `geekai_ai_model_price_config` (`model`, `module`, `credit_per_image`, `description`) VALUES
('vision-copywrite', 'copywrite', 8, 'AI代写商品卖点（图片分析，必须上传参考图）');

-- Task 1.5: AI 模型配置表
CREATE TABLE IF NOT EXISTS `geekai_ai_models` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name`         VARCHAR(64)     NOT NULL UNIQUE COMMENT '模型标识，如 NanoBanana、GPT-Image-2',
  `display_name` VARCHAR(128)    NOT NULL COMMENT '前端展示名称',
  `provider`     VARCHAR(64)     NOT NULL COMMENT '提供商，如 openai、aliyun、custom',
  `model_type`   VARCHAR(32)     NOT NULL DEFAULT 'image' COMMENT 'image|text|video',
  `api_endpoint` VARCHAR(512)    NULL     COMMENT 'API 接入地址（可选）',
  `api_key`      VARCHAR(512)    NULL     COMMENT 'API 密钥（可选，敏感字段）',
  `capabilities` VARCHAR(128)    NOT NULL DEFAULT '' COMMENT '模型能力，逗号分隔，如 text2img,img2img',
  `description`  VARCHAR(512)    NULL,
  `sort_order`   INT             NOT NULL DEFAULT 0 COMMENT '排序权重，越小越靠前',
  `status`       VARCHAR(16)     NOT NULL DEFAULT 'active' COMMENT 'active|disabled',
  `created_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_model_name` (`name`),
  KEY `idx_status_sort` (`status`, `sort_order`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI 模型配置表';

-- 初始模型数据
INSERT IGNORE INTO `geekai_ai_models` (`name`, `display_name`, `provider`, `model_type`, `capabilities`, `description`, `sort_order`) VALUES
('nano-banana',      'Nano Banana',         'relay', 'image', 'text2img,img2img', 'Gemini 优化版图像生成，支持文生图和图生图', 1),
('gpt-image-2',      'GPT-Image-2',         'relay', 'image', 'text2img',         'OpenAI GPT-Image-2 图像生成模型', 2),
('qwen-image-edit',  '通义万相·图生图',      'relay', 'image', 'img2img',          '阿里云通义万相图像编辑，仅支持图生图', 3);
