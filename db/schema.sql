CREATE TABLE `checksum` (
  `fid` bigint(20) unsigned NOT NULL,
  `hashtype` tinyint(3) unsigned NOT NULL,
  `checksum` varbinary(64) NOT NULL,
  PRIMARY KEY (`fid`),
  KEY `checksum_ndx` (`checksum`)
) ENGINE=InnoDB;
CREATE TABLE `class` (
  `dmid` smallint(5) unsigned NOT NULL,
  `classid` tinyint(3) unsigned NOT NULL,
  `classname` varchar(50) DEFAULT NULL,
  `mindevcount` tinyint(3) unsigned NOT NULL,
  `replpolicy` varchar(255) DEFAULT NULL,
  `hashtype` tinyint(3) unsigned DEFAULT NULL,
  PRIMARY KEY (`dmid`,`classid`),
  UNIQUE KEY `dmid` (`dmid`,`classname`)
) ENGINE=InnoDB;
CREATE TABLE `device` (
  `devid` mediumint(8) unsigned NOT NULL,
  `hostid` mediumint(8) unsigned NOT NULL,
  `status` enum('alive','dead','down','readonly','drain') DEFAULT NULL,
  `weight` mediumint(9) DEFAULT '100',
  `mb_total` int(10) unsigned DEFAULT NULL,
  `mb_used` int(10) unsigned DEFAULT NULL,
  `mb_asof` int(10) unsigned DEFAULT NULL,
  `io_utilization` tinyint(3) unsigned DEFAULT NULL,
  PRIMARY KEY (`devid`),
  KEY `status` (`status`)
) ENGINE=InnoDB;
CREATE TABLE `domain` (
  `dmid` smallint(5) unsigned NOT NULL,
  `namespace` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`dmid`),
  UNIQUE KEY `namespace` (`namespace`)
) ENGINE=InnoDB;
CREATE TABLE `file` (
  `fid` bigint(10) unsigned NOT NULL,
  `dmid` smallint(5) unsigned NOT NULL,
  `dkey` varchar(255) DEFAULT NULL,
  `length` bigint(20) unsigned DEFAULT NULL,
  `classid` tinyint(3) unsigned NOT NULL,
  `devcount` tinyint(3) unsigned NOT NULL,
  PRIMARY KEY (`fid`),
  UNIQUE KEY `dkey` (`dmid`,`dkey`),
  KEY `devcount` (`dmid`,`classid`,`devcount`)
) ENGINE=InnoDB;
CREATE TABLE `file_on` (
  `fid` bigint(20) unsigned NOT NULL,
  `devid` mediumint(8) unsigned NOT NULL,
  PRIMARY KEY (`fid`,`devid`),
  KEY `devid` (`devid`)
) ENGINE=InnoDB;
CREATE TABLE `file_on_corrupt` (
  `fid` bigint(10) unsigned NOT NULL,
  `devid` mediumint(8) unsigned NOT NULL,
  PRIMARY KEY (`fid`,`devid`)
) ENGINE=InnoDB;
CREATE TABLE `file_to_delete` (
  `fid` bigint(10) unsigned NOT NULL,
  PRIMARY KEY (`fid`)
) ENGINE=InnoDB;
CREATE TABLE `file_to_delete2` (
  `fid` bigint(10) unsigned NOT NULL,
  `nexttry` int(10) unsigned NOT NULL,
  `failcount` tinyint(3) unsigned NOT NULL DEFAULT '0',
  PRIMARY KEY (`fid`),
  KEY `nexttry` (`nexttry`)
) ENGINE=InnoDB;
CREATE TABLE `file_to_delete_later` (
  `fid` bigint(10) unsigned NOT NULL,
  `delafter` int(10) unsigned NOT NULL,
  PRIMARY KEY (`fid`),
  KEY `delafter` (`delafter`)
) ENGINE=InnoDB;
CREATE TABLE `file_to_queue` (
  `fid` bigint(10) unsigned NOT NULL,
  `devid` int(10) unsigned DEFAULT NULL,
  `type` tinyint(3) unsigned NOT NULL,
  `nexttry` int(10) unsigned NOT NULL,
  `failcount` tinyint(3) unsigned NOT NULL DEFAULT '0',
  `flags` smallint(5) unsigned NOT NULL DEFAULT '0',
  `arg` text,
  PRIMARY KEY (`fid`,`type`),
  KEY `type_nexttry` (`type`,`nexttry`)
) ENGINE=InnoDB;
CREATE TABLE `file_to_replicate` (
  `fid` bigint(10) unsigned NOT NULL,
  `nexttry` int(10) unsigned NOT NULL,
  `fromdevid` int(10) unsigned DEFAULT NULL,
  `failcount` tinyint(3) unsigned NOT NULL DEFAULT '0',
  `flags` smallint(5) unsigned NOT NULL DEFAULT '0',
  PRIMARY KEY (`fid`),
  KEY `nexttry` (`nexttry`)
) ENGINE=InnoDB;
CREATE TABLE `fixtures` (
  `name` varchar(255) DEFAULT NULL
) ENGINE=InnoDB;
CREATE TABLE `fsck_log` (
  `logid` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `utime` int(10) unsigned NOT NULL,
  `fid` bigint(20) unsigned DEFAULT NULL,
  `evcode` char(4) DEFAULT NULL,
  `devid` mediumint(8) unsigned DEFAULT NULL,
  PRIMARY KEY (`logid`),
  KEY `utime` (`utime`)
) ENGINE=InnoDB;
CREATE TABLE `host` (
  `hostid` mediumint(8) unsigned NOT NULL,
  `status` enum('alive','dead','down') DEFAULT NULL,
  `http_port` mediumint(8) unsigned DEFAULT '7500',
  `http_get_port` mediumint(8) unsigned DEFAULT NULL,
  `hostname` varchar(40) DEFAULT NULL,
  `hostip` varchar(40) DEFAULT NULL,
  `altip` varchar(15) DEFAULT NULL,
  `altmask` varchar(18) DEFAULT NULL,
  PRIMARY KEY (`hostid`),
  UNIQUE KEY `hostname` (`hostname`),
  UNIQUE KEY `hostip` (`hostip`),
  UNIQUE KEY `altip` (`altip`)
) ENGINE=InnoDB;
CREATE TABLE `migrations` (
  `name` varchar(255) DEFAULT NULL
) ENGINE=InnoDB;
CREATE TABLE `server_settings` (
  `field` varchar(50) NOT NULL,
  `value` text,
  PRIMARY KEY (`field`)
) ENGINE=InnoDB;
CREATE TABLE `tempfile` (
  `fid` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `createtime` int(10) unsigned NOT NULL,
  `classid` tinyint(3) unsigned NOT NULL,
  `dmid` smallint(5) unsigned NOT NULL,
  `dkey` varchar(255) DEFAULT NULL,
  `devids` varchar(5000) DEFAULT NULL,
  PRIMARY KEY (`fid`),
  KEY `ndx_createtime` (`createtime`)
) ENGINE=InnoDB AUTO_INCREMENT=390;
CREATE TABLE `unreachable_fids` (
  `fid` bigint(20) unsigned NOT NULL,
  `lastupdate` int(10) unsigned NOT NULL,
  PRIMARY KEY (`fid`),
  KEY `lastupdate` (`lastupdate`)
) ENGINE=InnoDB;
