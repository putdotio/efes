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
