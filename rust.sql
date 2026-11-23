/*
 Navicat Premium Dump SQL

 Source Server         : CarlJoy-MySQL
 Source Server Type    : MySQL
 Source Server Version : 50744 (5.7.44)
 Source Host           : carljoy.cn:3307
 Source Schema         : rust

 Target Server Type    : MySQL
 Target Server Version : 50744 (5.7.44)
 File Encoding         : 65001

 Date: 21/04/2026 15:40:14
*/

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ----------------------------
-- Table structure for player_bans
-- ----------------------------
DROP TABLE IF EXISTS `player_bans`;
CREATE TABLE `player_bans` (
  `id` int(11) NOT NULL AUTO_INCREMENT COMMENT '编号',
  `steam_id` varchar(32) NOT NULL COMMENT '被封禁玩家的SteamID',
  `player_name` varchar(100) DEFAULT NULL COMMENT '被封禁玩家的名字',
  `ip_address` varchar(64) DEFAULT NULL COMMENT '被封禁玩家的IP地址',
  `reason` varchar(255) DEFAULT NULL COMMENT '封禁原因',
  `banned_by_id` varchar(32) DEFAULT NULL COMMENT '执行封禁的管理员ID',
  `banned_by_name` varchar(100) DEFAULT NULL COMMENT '执行封禁的管理员名字',
  `ban_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '封禁时间',
  `expire_time` timestamp NULL DEFAULT NULL COMMENT '过期时间，NULL表示永久封禁',
  `unban_time` timestamp NULL DEFAULT NULL COMMENT '解封时间，如果不为空说明已解封',
  `unbanned_by_id` varchar(32) DEFAULT NULL COMMENT '执行解封的管理员ID',
  `unbanned_by_name` varchar(100) DEFAULT NULL COMMENT '执行解封的管理员名字',
  `server_name` varchar(100) DEFAULT NULL COMMENT '来源服务器名称',
  PRIMARY KEY (`id`),
  KEY `idx_steam_id` (`steam_id`),
  KEY `idx_ip_address` (`ip_address`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COMMENT='玩家封禁记录表';

-- ----------------------------
-- Table structure for player_reports
-- ----------------------------
DROP TABLE IF EXISTS `player_reports`;
CREATE TABLE `player_reports` (
  `id` int(11) NOT NULL AUTO_INCREMENT COMMENT '编号',
  `reporter_id` varchar(20) NOT NULL COMMENT '举报者 Steam ID',
  `reporter_name` varchar(100) NOT NULL COMMENT '举报者名称',
  `reporter_ip` varchar(45) DEFAULT NULL COMMENT '举报者 IP',
  `suspect_id` varchar(20) NOT NULL COMMENT '被举报者 Steam ID',
  `suspect_name` varchar(100) NOT NULL COMMENT '被举报者名称',
  `suspect_ip` varchar(45) DEFAULT NULL COMMENT '被举报者 IP',
  `report_subject` varchar(255) DEFAULT NULL COMMENT '举报主题',
  `report_message` text NOT NULL COMMENT '举报内容',
  `report_type` varchar(50) DEFAULT NULL COMMENT '举报类型(cheat/abusive/name/spam)',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `server_name` varchar(255) DEFAULT NULL COMMENT '服务器',
  PRIMARY KEY (`id`),
  KEY `idx_reporter` (`reporter_id`),
  KEY `idx_suspect` (`suspect_id`),
  KEY `idx_created` (`created_at`)
) ENGINE=InnoDB AUTO_INCREMENT=9 DEFAULT CHARSET=utf8mb4 COMMENT='玩家举报记录表';

-- ----------------------------
-- Table structure for stats_player_data
-- ----------------------------
DROP TABLE IF EXISTS `stats_player_data`;
CREATE TABLE `stats_player_data` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `player_id` varchar(32) NOT NULL,
  `player_name` varchar(100) DEFAULT NULL,
  `server_name` varchar(100) NOT NULL,
  `wipe_date` date NOT NULL,
  `pvp_kills` int(11) DEFAULT '0',
  `pvp_deaths` int(11) DEFAULT '0',
  `pvp_suicides` int(11) DEFAULT '0',
  `pvp_headshots` int(11) DEFAULT '0',
  `pvp_wounded` int(11) DEFAULT '0',
  `pvp_shots_fired` int(11) DEFAULT '0',
  `pvp_shots_hit` int(11) DEFAULT '0',
  `pvp_longest_kill` float DEFAULT '0',
  `pvp_kdr` float DEFAULT '0',
  `res_sulfur_ore` int(11) DEFAULT '0',
  `res_metal_ore` int(11) DEFAULT '0',
  `res_hq_metal_ore` int(11) DEFAULT '0',
  `res_stone` int(11) DEFAULT '0',
  `res_wood` int(11) DEFAULT '0',
  `res_leather` int(11) DEFAULT '0',
  `res_animal_fat` int(11) DEFAULT '0',
  `res_bone_fragments` int(11) DEFAULT '0',
  `raid_c4` int(11) DEFAULT '0',
  `raid_beancan` int(11) DEFAULT '0',
  `raid_hv_rocket` int(11) DEFAULT '0',
  `raid_rocket` int(11) DEFAULT '0',
  `farm_cloth` int(11) DEFAULT '0',
  `farm_berry_blue` int(11) DEFAULT '0',
  `farm_berry_green` int(11) DEFAULT '0',
  `farm_berry_red` int(11) DEFAULT '0',
  `farm_berry_white` int(11) DEFAULT '0',
  `farm_berry_yellow` int(11) DEFAULT '0',
  `farm_berry_black` int(11) DEFAULT '0',
  `farm_mushroom` int(11) DEFAULT '0',
  `farm_pumpkin` int(11) DEFAULT '0',
  `farm_corn` int(11) DEFAULT '0',
  `farm_potato` int(11) DEFAULT '0',
  `farm_wheat` int(11) DEFAULT '0',
  `farm_flowers` int(11) DEFAULT '0',
  `farm_eggs` int(11) DEFAULT '0',
  `online_time_seconds` bigint(20) DEFAULT '0',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `unique_stats` (`player_id`,`server_name`,`wipe_date`)
) ENGINE=InnoDB AUTO_INCREMENT=33324 DEFAULT CHARSET=utf8mb4 COMMENT='玩家数据统计';

SET FOREIGN_KEY_CHECKS = 1;
