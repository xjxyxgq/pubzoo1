/*
 Navicat Premium Dump SQL

 Source Server         : localhost_3311
 Source Server Type    : MySQL
 Source Server Version : 50732 (5.7.32-debug-log)
 Source Host           : localhost:3311
 Source Schema         : cmdb2

 Target Server Type    : MySQL
 Target Server Version : 50732 (5.7.32-debug-log)
 File Encoding         : 65001

 Date: 07/08/2025 16:52:30
*/

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ----------------------------
-- Table structure for application_collection_config
-- ----------------------------
DROP TABLE IF EXISTS `application_collection_config`;
CREATE TABLE `application_collection_config` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `app_type` varchar(50) NOT NULL COMMENT '应用类型(mysql,tidb,goldendb,mssql等)',
  `app_name` varchar(100) NOT NULL COMMENT '应用名称',
  `source_table` varchar(100) NOT NULL COMMENT '源数据表名',
  `collection_script_path` varchar(500) DEFAULT NULL COMMENT '采集脚本路径',
  `collection_method` enum('database','script','api') NOT NULL DEFAULT 'database' COMMENT '采集方式：database-数据库查询，script-脚本执行，api-接口调用',
  `collection_config` json DEFAULT NULL COMMENT '采集配置（JSON格式）',
  `output_format` json DEFAULT NULL COMMENT '输出格式配置（JSON格式）',
  `is_enabled` tinyint(1) NOT NULL DEFAULT '1' COMMENT '是否启用',
  `description` text COMMENT '描述信息',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_app_type` (`app_type`),
  KEY `idx_app_name` (`app_name`),
  KEY `idx_enabled` (`is_enabled`)
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8mb4 COMMENT='应用采集配置表';

-- ----------------------------
-- Table structure for application_collection_result
-- ----------------------------
DROP TABLE IF EXISTS `application_collection_result`;
CREATE TABLE `application_collection_result` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `host_ip` varchar(45) NOT NULL COMMENT '主机IP',
  `hostname` varchar(100) DEFAULT NULL COMMENT '主机名',
  `app_type` varchar(50) NOT NULL COMMENT '应用类型',
  `app_name` varchar(100) NOT NULL COMMENT '应用名称',
  `cluster_name` varchar(100) DEFAULT NULL COMMENT '集群名称',
  `instance_role` varchar(50) DEFAULT NULL COMMENT '实例角色',
  `port` int(11) DEFAULT NULL COMMENT '端口',
  `version` varchar(50) DEFAULT NULL COMMENT '版本信息',
  `status` varchar(50) DEFAULT NULL COMMENT '状态',
  `business_line` varchar(100) DEFAULT NULL COMMENT '业务线',
  `department` varchar(100) DEFAULT NULL COMMENT '部门',
  `data_dir` varchar(500) DEFAULT NULL COMMENT '数据目录',
  `config_info` json DEFAULT NULL COMMENT '其他配置信息（JSON格式）',
  `collection_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '采集时间',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_host_ip` (`host_ip`),
  KEY `idx_app_type` (`app_type`),
  KEY `idx_cluster_name` (`cluster_name`),
  KEY `idx_collection_time` (`collection_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='应用采集结果表';

-- ----------------------------
-- Table structure for backup_restore_check_info
-- ----------------------------
DROP TABLE IF EXISTS `backup_restore_check_info`;
CREATE TABLE `backup_restore_check_info` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `check_seq` varchar(20) NOT NULL COMMENT '检查轮次',
  `check_db` varchar(50) NOT NULL COMMENT '检查数据库集群',
  `check_src_ip` varchar(50) NOT NULL COMMENT '备份IP',
  `db_backup_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '数据库备份时间',
  `db_backup_date` varchar(20) NOT NULL COMMENT '备份日期',
  `backup_name` varchar(200) NOT NULL COMMENT '备份文件名',
  `db_restore_begin_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '恢复开始时间',
  `check_dst_ip` varchar(50) NOT NULL COMMENT '恢复IP',
  `check_app` varchar(50) NOT NULL COMMENT '恢复业务',
  `check_db_type` varchar(20) NOT NULL COMMENT '数据库架构',
  `db_app_line` varchar(50) NOT NULL COMMENT '数据库应用团队',
  `db_restore_end_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '恢复结束时间',
  `backup_check_result` varchar(10) NOT NULL DEFAULT 'OK' COMMENT '备份检查结果',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `delete_time` datetime DEFAULT NULL,
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `idx_check_seq` (`check_seq`),
  KEY `idx_check_db` (`check_db`),
  KEY `idx_check_src_ip` (`check_src_ip`),
  KEY `idx_db_backup_date` (`db_backup_date`)
) ENGINE=InnoDB AUTO_INCREMENT=6 DEFAULT CHARSET=utf8mb4 COMMENT='备份恢复检查信息表';

-- ----------------------------
-- Table structure for cluster_groups
-- ----------------------------
DROP TABLE IF EXISTS `cluster_groups`;
CREATE TABLE `cluster_groups` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `delete_time` datetime DEFAULT NULL,
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0',
  `group_name` varchar(100) NOT NULL COMMENT '组名称',
  `cluster_name` varchar(64) NOT NULL COMMENT '集群名称',
  `department_line_name` varchar(100) NOT NULL COMMENT '部门业务线名称',
  `cluster_type` varchar(30) DEFAULT NULL COMMENT '集群类型',
  PRIMARY KEY (`id`),
  KEY `idx_group_name` (`group_name`),
  KEY `idx_cluster_name` (`cluster_name`),
  KEY `idx_department_name` (`department_line_name`)
) ENGINE=InnoDB AUTO_INCREMENT=53 DEFAULT CHARSET=utf8mb4 COMMENT='集群组表';

-- ----------------------------
-- Table structure for db_line
-- ----------------------------
DROP TABLE IF EXISTS `db_line`;
CREATE TABLE `db_line` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `cluster_group_name` varchar(100) NOT NULL COMMENT '集群组名称',
  `department_line_name` varchar(100) NOT NULL COMMENT '部门业务线名称',
  `business_domain` varchar(100) DEFAULT NULL COMMENT '业务域',
  `contact_person` varchar(50) DEFAULT NULL COMMENT '联系人',
  `contact_email` varchar(100) DEFAULT NULL COMMENT '联系邮箱',
  `description` text COMMENT '描述',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `delete_time` datetime DEFAULT NULL,
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_cluster_group_department` (`cluster_group_name`,`department_line_name`),
  KEY `idx_cluster_group_name` (`cluster_group_name`),
  KEY `idx_department_line_name` (`department_line_name`)
) ENGINE=InnoDB AUTO_INCREMENT=19 DEFAULT CHARSET=utf8mb4 COMMENT='集群组与业务线关系表';

-- ----------------------------
-- Table structure for goldendb_cluster
-- ----------------------------
DROP TABLE IF EXISTS `goldendb_cluster`;
CREATE TABLE `goldendb_cluster` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `cluster_name` varchar(64) NOT NULL COMMENT '集群名称',
  `cluster_group_name` varchar(100) NOT NULL COMMENT '集群组名称',
  `cluster_type` varchar(20) DEFAULT 'goldendb' COMMENT '集群类型',
  `cluster_status` varchar(20) DEFAULT 'active' COMMENT '集群状态',
  `description` text COMMENT '集群描述',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `delete_time` datetime DEFAULT NULL,
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_cluster_name` (`cluster_name`),
  KEY `idx_cluster_group_name` (`cluster_group_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='GoldenDB集群表';

-- ----------------------------
-- Table structure for goldendb_cluster_instance
-- ----------------------------
DROP TABLE IF EXISTS `goldendb_cluster_instance`;
CREATE TABLE `goldendb_cluster_instance` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `cluster_name` varchar(64) NOT NULL COMMENT '集群名称',
  `ip` varchar(50) NOT NULL COMMENT '实例IP',
  `port` int(11) NOT NULL DEFAULT '3306' COMMENT '实例端口',
  `instance_role` varchar(20) NOT NULL COMMENT '实例角色(primary/secondary)',
  `version` varchar(30) DEFAULT NULL COMMENT 'GoldenDB版本',
  `instance_status` varchar(20) DEFAULT 'running' COMMENT '实例状态',
  `data_dir` varchar(255) DEFAULT NULL COMMENT '数据目录',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `delete_time` datetime DEFAULT NULL,
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_cluster_ip_port` (`cluster_name`,`ip`,`port`),
  KEY `idx_cluster_name` (`cluster_name`),
  KEY `idx_ip` (`ip`),
  KEY `idx_instance_role` (`instance_role`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='GoldenDB集群实例表';

-- ----------------------------
-- Table structure for hardware_resource_verification
-- ----------------------------
DROP TABLE IF EXISTS `hardware_resource_verification`;
CREATE TABLE `hardware_resource_verification` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `delete_time` datetime DEFAULT NULL,
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0',
  `task_id` varchar(64) NOT NULL COMMENT '任务ID，用于关联同一批次的多个主机验证',
  `host_ip` varchar(50) NOT NULL COMMENT '目标主机IP',
  `resource_type` enum('cpu','memory','disk') NOT NULL COMMENT '资源类型',
  `target_percent` int(10) unsigned NOT NULL COMMENT '目标资源占用百分比',
  `duration` int(10) unsigned NOT NULL DEFAULT '300' COMMENT '执行持续时间（秒）',
  `script_params` text COMMENT '脚本执行参数（JSON格式）',
  `execution_status` enum('pending','running','completed','failed') NOT NULL DEFAULT 'pending' COMMENT '执行状态',
  `start_time` datetime DEFAULT NULL COMMENT '执行开始时间',
  `end_time` datetime DEFAULT NULL COMMENT '执行结束时间',
  `exit_code` int(11) DEFAULT NULL COMMENT '脚本退出代码',
  `stdout_log` longtext COMMENT '标准输出日志',
  `stderr_log` longtext COMMENT '错误输出日志',
  `result_summary` text COMMENT '执行结果摘要（JSON格式）',
  `ssh_error` text COMMENT 'SSH连接错误信息',
  PRIMARY KEY (`id`),
  KEY `idx_task_id` (`task_id`),
  KEY `idx_host_ip` (`host_ip`),
  KEY `idx_resource_type` (`resource_type`),
  KEY `idx_execution_status` (`execution_status`),
  KEY `idx_created_at` (`create_time`),
  KEY `idx_host_resource_status` (`host_ip`,`resource_type`,`execution_status`),
  KEY `idx_task_resource_time` (`task_id`,`resource_type`,`create_time`)
) ENGINE=InnoDB AUTO_INCREMENT=80 DEFAULT CHARSET=utf8mb4 COMMENT='硬件资源验证执行记录表';

-- ----------------------------
-- Table structure for hosts_applications
-- ----------------------------
DROP TABLE IF EXISTS `hosts_applications`;
CREATE TABLE `hosts_applications` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `delete_time` datetime DEFAULT NULL,
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0',
  `pool_id` int(10) unsigned NOT NULL COMMENT '主机池ID',
  `server_type` varchar(30) DEFAULT NULL COMMENT '服务类型(mysql/mssql/other)',
  `server_version` varchar(30) DEFAULT NULL COMMENT '服务版本',
  `server_subtitle` varchar(30) DEFAULT NULL COMMENT '服务子标题',
  `cluster_name` varchar(64) DEFAULT NULL COMMENT '集群名称',
  `server_protocol` varchar(64) DEFAULT NULL COMMENT '服务协议',
  `server_addr` varchar(100) DEFAULT NULL COMMENT '服务地址',
  `server_port` int(11) NOT NULL COMMENT '服务端口',
  `server_role` varchar(100) DEFAULT NULL COMMENT '服务角色',
  `server_status` varchar(100) DEFAULT NULL COMMENT '服务状态',
  `department_name` varchar(100) DEFAULT NULL COMMENT '部门名称',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_app` (`pool_id`,`server_type`,`cluster_name`,`server_port`) USING BTREE,
  KEY `idx_pool_id` (`pool_id`),
  KEY `idx_server_type` (`server_type`),
  KEY `idx_cluster_name` (`cluster_name`),
  KEY `idx_deleted_at` (`delete_time`),
  CONSTRAINT `fk_hosts_applications_pool_id` FOREIGN KEY (`pool_id`) REFERENCES `hosts_pool` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=21 DEFAULT CHARSET=utf8mb4 COMMENT='主机应用部署表';

-- ----------------------------
-- Table structure for hosts_pool
-- ----------------------------
DROP TABLE IF EXISTS `hosts_pool`;
CREATE TABLE `hosts_pool` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `host_name` varchar(50) NOT NULL COMMENT '主机名',
  `host_ip` varchar(50) NOT NULL COMMENT '主机IP',
  `host_type` varchar(10) DEFAULT NULL COMMENT '主机类型',
  `h3c_id` varchar(50) DEFAULT NULL COMMENT 'H3C ID',
  `h3c_status` varchar(20) DEFAULT NULL COMMENT 'H3C状态',
  `disk_size` int(10) unsigned DEFAULT NULL COMMENT '磁盘大小(GB)',
  `ram` int(10) unsigned DEFAULT NULL COMMENT '内存大小(GB)',
  `vcpus` int(10) unsigned DEFAULT NULL COMMENT 'CPU核数',
  `if_h3c_sync` varchar(10) DEFAULT NULL COMMENT '是否H3C同步',
  `h3c_img_id` varchar(50) DEFAULT NULL COMMENT 'H3C镜像ID',
  `h3c_hm_name` varchar(1000) DEFAULT NULL COMMENT 'H3C主机名',
  `is_delete` varchar(10) DEFAULT NULL COMMENT '是否删除标记',
  `leaf_number` varchar(50) DEFAULT NULL COMMENT '叶子节点编号',
  `rack_number` varchar(10) DEFAULT NULL COMMENT '机架号',
  `rack_height` int(10) unsigned DEFAULT NULL COMMENT '机架高度',
  `rack_start_number` int(10) unsigned DEFAULT NULL COMMENT '机架起始位置',
  `from_factor` int(10) unsigned DEFAULT NULL COMMENT '规格因子',
  `serial_number` varchar(50) DEFAULT NULL COMMENT '序列号',
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0' COMMENT '是否删除',
  `is_static` tinyint(1) NOT NULL DEFAULT '0' COMMENT '是否静态',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `delete_time` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_host_ip` (`host_ip`),
  KEY `idx_host_name` (`host_name`),
  KEY `idx_host_type` (`host_type`),
  KEY `idx_is_deleted` (`is_deleted`)
) ENGINE=InnoDB AUTO_INCREMENT=6 DEFAULT CHARSET=utf8mb4 COMMENT='主机资源池表';

-- ----------------------------
-- Table structure for mssql_cluster
-- ----------------------------
DROP TABLE IF EXISTS `mssql_cluster`;
CREATE TABLE `mssql_cluster` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `cluster_name` varchar(64) NOT NULL COMMENT '集群名称',
  `cluster_group_name` varchar(100) NOT NULL COMMENT '集群组名称',
  `cluster_type` varchar(20) DEFAULT 'mssql' COMMENT '集群类型',
  `cluster_status` varchar(20) DEFAULT 'active' COMMENT '集群状态',
  `description` text COMMENT '集群描述',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `delete_time` datetime DEFAULT NULL,
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_cluster_name` (`cluster_name`),
  KEY `idx_cluster_group_name` (`cluster_group_name`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COMMENT='MSSQL集群表';

-- ----------------------------
-- Table structure for mssql_cluster_instance
-- ----------------------------
DROP TABLE IF EXISTS `mssql_cluster_instance`;
CREATE TABLE `mssql_cluster_instance` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `cluster_name` varchar(64) NOT NULL COMMENT '集群名称',
  `ip` varchar(50) NOT NULL COMMENT '实例IP',
  `instance_port` int(11) NOT NULL DEFAULT '1433' COMMENT '实例端口',
  `instance_role` varchar(20) NOT NULL COMMENT '实例角色(primary/secondary)',
  `version` varchar(30) DEFAULT NULL COMMENT 'MSSQL版本',
  `instance_status` varchar(20) DEFAULT 'running' COMMENT '实例状态',
  `data_dir` varchar(255) DEFAULT NULL COMMENT '数据目录',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `delete_time` datetime DEFAULT NULL,
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_cluster_ip_port` (`cluster_name`,`ip`,`instance_port`),
  KEY `idx_cluster_name` (`cluster_name`),
  KEY `idx_ip` (`ip`),
  KEY `idx_instance_role` (`instance_role`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COMMENT='MSSQL集群实例表';

-- ----------------------------
-- Table structure for mysql_cluster
-- ----------------------------
DROP TABLE IF EXISTS `mysql_cluster`;
CREATE TABLE `mysql_cluster` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `cluster_name` varchar(64) NOT NULL COMMENT '集群名称',
  `cluster_group_name` varchar(100) NOT NULL COMMENT '集群组名称',
  `cluster_type` varchar(20) DEFAULT 'mysql' COMMENT '集群类型',
  `cluster_status` varchar(20) DEFAULT 'active' COMMENT '集群状态',
  `description` text COMMENT '集群描述',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `delete_time` datetime DEFAULT NULL,
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_cluster_name` (`cluster_name`),
  KEY `idx_cluster_group_name` (`cluster_group_name`)
) ENGINE=InnoDB AUTO_INCREMENT=16 DEFAULT CHARSET=utf8mb4 COMMENT='MySQL集群表';

-- ----------------------------
-- Table structure for mysql_cluster_instance
-- ----------------------------
DROP TABLE IF EXISTS `mysql_cluster_instance`;
CREATE TABLE `mysql_cluster_instance` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `cluster_name` varchar(64) NOT NULL COMMENT '集群名称',
  `ip` varchar(50) NOT NULL COMMENT '实例IP',
  `port` int(11) NOT NULL DEFAULT '3306' COMMENT '实例端口',
  `instance_role` varchar(20) NOT NULL COMMENT '实例角色(master/slave)',
  `version` varchar(30) DEFAULT NULL COMMENT 'MySQL版本',
  `instance_status` varchar(20) DEFAULT 'running' COMMENT '实例状态',
  `data_dir` varchar(255) DEFAULT NULL COMMENT '数据目录',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `delete_time` datetime DEFAULT NULL,
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_cluster_ip_port` (`cluster_name`,`ip`,`port`),
  KEY `idx_cluster_name` (`cluster_name`),
  KEY `idx_ip` (`ip`),
  KEY `idx_instance_role` (`instance_role`)
) ENGINE=InnoDB AUTO_INCREMENT=22 DEFAULT CHARSET=utf8mb4 COMMENT='MySQL集群实例表';

-- ----------------------------
-- Table structure for plugin_execution_records
-- ----------------------------
DROP TABLE IF EXISTS `plugin_execution_records`;
CREATE TABLE `plugin_execution_records` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `delete_time` datetime DEFAULT NULL,
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0',
  `check_seq` varchar(20) NOT NULL COMMENT '检查轮次',
  `plugin_name` varchar(100) NOT NULL COMMENT '插件名称',
  `execution_log` text COMMENT '执行日志',
  `result` text COMMENT '执行结果',
  PRIMARY KEY (`id`),
  KEY `idx_check_seq` (`check_seq`),
  KEY `idx_plugin_name` (`plugin_name`),
  KEY `idx_deleted_at` (`delete_time`)
) ENGINE=InnoDB AUTO_INCREMENT=9 DEFAULT CHARSET=utf8mb4 COMMENT='插件执行记录表';

-- ----------------------------
-- Table structure for resource_analysis_reports
-- ----------------------------
DROP TABLE IF EXISTS `resource_analysis_reports`;
CREATE TABLE `resource_analysis_reports` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `delete_time` datetime DEFAULT NULL,
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0',
  `resource_usage_id` int(10) unsigned NOT NULL COMMENT '资源使用数据ID',
  `analysis_request_id` varchar(100) NOT NULL COMMENT '分析请求ID',
  `analysis_report` text NOT NULL COMMENT '分析报告内容',
  PRIMARY KEY (`id`),
  KEY `idx_resource_usage_id` (`resource_usage_id`),
  KEY `idx_analysis_request_id` (`analysis_request_id`),
  KEY `idx_deleted_at` (`delete_time`),
  CONSTRAINT `fk_resource_analysis_resource_usage` FOREIGN KEY (`resource_usage_id`) REFERENCES `resource_usage_data` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=13 DEFAULT CHARSET=utf8mb4 COMMENT='资源分析报告表';

-- ----------------------------
-- Table structure for scheduled_hardware_verification
-- ----------------------------
DROP TABLE IF EXISTS `scheduled_hardware_verification`;
CREATE TABLE `scheduled_hardware_verification` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `task_name` varchar(255) NOT NULL COMMENT '任务名称',
  `description` text COMMENT '任务描述',
  `cron_expression` varchar(100) NOT NULL COMMENT 'Cron表达式',
  `host_ip_list` text NOT NULL COMMENT '主机IP列表（JSON格式）',
  `resource_type` enum('cpu','memory','disk') NOT NULL COMMENT '资源类型',
  `target_percent` int(11) NOT NULL COMMENT '目标百分比',
  `duration` int(11) NOT NULL DEFAULT '300' COMMENT '执行持续时间（秒）',
  `script_params` text COMMENT '脚本参数（JSON格式）',
  `force_execution` tinyint(1) DEFAULT '0' COMMENT '是否强制执行',
  `is_enabled` tinyint(1) DEFAULT '1' COMMENT '是否启用',
  `created_by` varchar(100) DEFAULT NULL COMMENT '创建者',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `delete_time` datetime DEFAULT NULL,
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0',
  `last_execution_time` timestamp NULL DEFAULT NULL COMMENT '上次执行时间',
  `next_execution_time` timestamp NULL DEFAULT NULL COMMENT '下次执行时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_task_name` (`task_name`)
) ENGINE=InnoDB AUTO_INCREMENT=8 DEFAULT CHARSET=utf8mb4 COMMENT='定时硬件资源验证任务配置表';

-- ----------------------------
-- Table structure for scheduled_task_execution_history
-- ----------------------------
DROP TABLE IF EXISTS `scheduled_task_execution_history`;
CREATE TABLE `scheduled_task_execution_history` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `scheduled_task_id` bigint(20) NOT NULL COMMENT '定时任务ID',
  `execution_task_id` varchar(255) NOT NULL COMMENT '执行任务ID（对应hardware_resource_verification.task_id）',
  `execution_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '执行时间',
  `execution_status` enum('success','failed','partial') NOT NULL COMMENT '执行状态',
  `total_hosts` int(11) NOT NULL COMMENT '总主机数',
  `success_hosts` int(11) DEFAULT '0' COMMENT '成功主机数',
  `failed_hosts` int(11) DEFAULT '0' COMMENT '失败主机数',
  `error_message` text COMMENT '错误信息',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `delete_time` datetime DEFAULT NULL,
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `idx_scheduled_task_id` (`scheduled_task_id`),
  KEY `idx_execution_time` (`execution_time`),
  CONSTRAINT `scheduled_task_execution_history_ibfk_1` FOREIGN KEY (`scheduled_task_id`) REFERENCES `scheduled_hardware_verification` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=26 DEFAULT CHARSET=utf8mb4 COMMENT='定时任务执行历史表';

-- ----------------------------
-- Table structure for server_resources
-- ----------------------------
DROP TABLE IF EXISTS `server_resources`;
CREATE TABLE `server_resources` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `delete_time` datetime DEFAULT NULL,
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0',
  `pool_id` int(10) unsigned NOT NULL COMMENT '主机池ID',
  `cluster_name` varchar(64) DEFAULT NULL COMMENT '集群名称',
  `group_name` varchar(100) DEFAULT NULL COMMENT '组名称',
  `ip` varchar(50) DEFAULT NULL COMMENT 'IP地址',
  `port` int(10) unsigned DEFAULT NULL COMMENT '端口',
  `instance_role` varchar(50) DEFAULT NULL COMMENT '实例角色',
  `total_memory` double DEFAULT NULL COMMENT '总内存(GB)',
  `used_memory` double DEFAULT NULL COMMENT '已用内存(GB)',
  `total_disk` double DEFAULT NULL COMMENT '总磁盘(GB)',
  `used_disk` double DEFAULT NULL COMMENT '已用磁盘(GB)',
  `cpu_cores` int(11) DEFAULT NULL COMMENT 'CPU核数',
  `cpu_load` double DEFAULT NULL COMMENT 'CPU负载(%)',
  `date_time` datetime NOT NULL COMMENT '监控时间',
  PRIMARY KEY (`id`),
  KEY `idx_pool_id` (`pool_id`),
  KEY `idx_cluster_name` (`cluster_name`),
  KEY `idx_ip` (`ip`),
  KEY `idx_date_time` (`date_time`),
  CONSTRAINT `fk_server_resources_pool_id` FOREIGN KEY (`pool_id`) REFERENCES `hosts_pool` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=1128 DEFAULT CHARSET=utf8mb4 COMMENT='服务器资源监控表';

-- ----------------------------
-- Table structure for tidb_cluster
-- ----------------------------
DROP TABLE IF EXISTS `tidb_cluster`;
CREATE TABLE `tidb_cluster` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `cluster_name` varchar(64) NOT NULL COMMENT '集群名称',
  `cluster_group_name` varchar(100) NOT NULL COMMENT '集群组名称',
  `cluster_type` varchar(20) DEFAULT 'tidb' COMMENT '集群类型',
  `cluster_status` varchar(20) DEFAULT 'active' COMMENT '集群状态',
  `description` text COMMENT '集群描述',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `delete_time` datetime DEFAULT NULL,
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_cluster_name` (`cluster_name`),
  KEY `idx_cluster_group_name` (`cluster_group_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='TiDB集群表';

-- ----------------------------
-- Table structure for tidb_cluster_instance
-- ----------------------------
DROP TABLE IF EXISTS `tidb_cluster_instance`;
CREATE TABLE `tidb_cluster_instance` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `cluster_name` varchar(64) NOT NULL COMMENT '集群名称',
  `ip` varchar(50) NOT NULL COMMENT '实例IP',
  `port` int(11) NOT NULL DEFAULT '4000' COMMENT '实例端口',
  `instance_role` varchar(20) NOT NULL COMMENT '实例角色(tidb/tikv/pd)',
  `version` varchar(30) DEFAULT NULL COMMENT 'TiDB版本',
  `instance_status` varchar(20) DEFAULT 'running' COMMENT '实例状态',
  `data_dir` varchar(255) DEFAULT NULL COMMENT '数据目录',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `delete_time` datetime DEFAULT NULL,
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_cluster_ip_port` (`cluster_name`,`ip`,`port`),
  KEY `idx_cluster_name` (`cluster_name`),
  KEY `idx_ip` (`ip`),
  KEY `idx_instance_role` (`instance_role`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='TiDB集群实例表';

-- ----------------------------
-- Table structure for user_sessions
-- ----------------------------
DROP TABLE IF EXISTS `user_sessions`;
CREATE TABLE `user_sessions` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `delete_time` datetime DEFAULT NULL,
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0',
  `user_id` int(10) unsigned NOT NULL COMMENT '用户ID',
  `session_token` varchar(500) NOT NULL COMMENT '会话令牌',
  `cas_ticket` varchar(500) DEFAULT NULL COMMENT 'CAS票据',
  `expires_at` datetime NOT NULL COMMENT '过期时间',
  `ip_address` varchar(45) DEFAULT NULL COMMENT 'IP地址',
  `user_agent` text COMMENT '用户代理',
  `is_active` tinyint(1) NOT NULL DEFAULT '1' COMMENT '是否有效',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_session_token` (`session_token`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_expires_at` (`expires_at`),
  KEY `idx_is_active` (`is_active`),
  CONSTRAINT `fk_user_sessions_user_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=82 DEFAULT CHARSET=utf8mb4 COMMENT='用户会话表';

-- ----------------------------
-- Table structure for users
-- ----------------------------
DROP TABLE IF EXISTS `users`;
CREATE TABLE `users` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `delete_time` datetime DEFAULT NULL,
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0',
  `username` varchar(50) NOT NULL COMMENT '用户名',
  `password_hash` varchar(255) DEFAULT NULL COMMENT '密码哈希（CAS模式下为空）',
  `email` varchar(100) DEFAULT NULL COMMENT '邮箱',
  `display_name` varchar(100) DEFAULT NULL COMMENT '显示名称',
  `is_active` tinyint(1) NOT NULL DEFAULT '1' COMMENT '是否激活',
  `is_admin` tinyint(1) NOT NULL DEFAULT '0' COMMENT '是否管理员',
  `last_login_at` datetime DEFAULT NULL COMMENT '最后登录时间',
  `login_source` enum('local','cas') NOT NULL DEFAULT 'local' COMMENT '登录来源',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_username` (`username`),
  KEY `idx_email` (`email`),
  KEY `idx_is_active` (`is_active`),
  KEY `idx_login_source` (`login_source`),
  KEY `idx_deleted_at` (`delete_time`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COMMENT='用户表';

-- ----------------------------
-- Records of users
-- ----------------------------
BEGIN;
INSERT INTO `users` (`id`, `create_time`, `update_time`, `delete_time`, `is_deleted`, `username`, `password_hash`, `email`, `display_name`, `is_active`, `is_admin`, `last_login_at`, `login_source`) VALUES (1, '2025-07-22 11:31:08', '2025-08-06 14:41:06', NULL, 0, 'admin', '$2a$10$rV8O4w3KUmFE3/W4zz2YBOuyD96FYtDaR4Oa4IB/piNEb0QCl6XhW', 'admin@example.com', '系统管理员', 1, 1, '2025-08-06 14:41:07', 'local');
INSERT INTO `users` (`id`, `create_time`, `update_time`, `delete_time`, `is_deleted`, `username`, `password_hash`, `email`, `display_name`, `is_active`, `is_admin`, `last_login_at`, `login_source`) VALUES (2, '2025-07-22 11:31:08', '2025-07-22 17:16:12', NULL, 0, 'test', '$2a$10$rV8O4w3KUmFE3/W4zz2YBOuyD96FYtDaR4Oa4IB/piNEb0QCl6XhW', 'test@example.com', '测试用户', 1, 0, NULL, 'local');
COMMIT;

SET FOREIGN_KEY_CHECKS = 1;
