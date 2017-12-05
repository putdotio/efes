SET default_storage_engine=INNODB;

CREATE TABLE `host` (
  `hostid` mediumint(8) unsigned NOT NULL,
  `status` enum('alive','dead','down') NOT NULL DEFAULT 'alive',
  `hostname` varchar(40) NOT NULL,
  `hostip` varchar(40) NOT NULL,
  PRIMARY KEY (`hostid`)
);

CREATE TABLE `device` (
  `devid` mediumint(8) unsigned NOT NULL,
  `hostid` mediumint(8) unsigned NOT NULL,
  `read_port` mediumint(8) unsigned NOT NULL DEFAULT '8500',
  `write_port` mediumint(8) unsigned NOT NULL DEFAULT '8501',
  `status` enum('alive','dead','down','drain') NOT NULL DEFAULT 'alive',
  `bytes_total` bigint(20) unsigned DEFAULT NULL,
  `bytes_used` bigint(20) unsigned DEFAULT NULL,
  `bytes_free` bigint(20) unsigned DEFAULT NULL,
  `io_utilization` tinyint(3) unsigned DEFAULT NULL,
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `last_disk_clean_time` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`devid`),
  FOREIGN KEY (`hostid`) REFERENCES `host` (`hostid`)
);

CREATE TABLE `file` (
  `fid` bigint(10) unsigned NOT NULL,
  `dkey` varchar(255) NOT NULL,
  PRIMARY KEY (`fid`),
  UNIQUE KEY `dkey` (`dkey`)
);

CREATE TABLE `tempfile` (
  `fid` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `devid` mediumint(8) unsigned NOT NULL,
  PRIMARY KEY (`fid`),
  FOREIGN KEY (`devid`) REFERENCES `device` (`devid`),
  KEY `ndx_created_at` (`created_at`)
);

CREATE TABLE `file_on` (
  `fid` bigint(20) unsigned NOT NULL,
  `devid` mediumint(8) unsigned NOT NULL,
  PRIMARY KEY (`fid`,`devid`),
  FOREIGN KEY (`fid`) REFERENCES `file` (`fid`),
  FOREIGN KEY (`devid`) REFERENCES `device` (`devid`)
);
