// Package siteimport brings websites over from other control panels: a
// full cPanel backup, or a site's files archive with SQL dumps (aaPanel,
// HestiaCP, CyberPanel, a plain server).
//
// Every archive comes from another server and is treated as hostile. An
// import first analyses the archive without unpacking it (it is read as a
// stream; only small metadata files are kept), and the administrator picks
// what to bring over from the resulting plan. The import then reads the
// archive once more and writes each website's files through an os.Root of
// that site's public folder, so no name can lead outside it: entries with
// ".." or absolute names, links, hard links, devices and FIFOs are left out
// and reported, modes lose setuid, setgid and group or world write, every
// file belongs to the site's user, and the bytes really written count
// against a per-file and a total limit (archive bombs stop there, as do
// gzip streams that inflate much more than any website does). SQL runs as
// the database package's limited import account, never as root, and no
// shell is involved anywhere. If anything fails, or the job is stopped,
// what the import created is removed; a site that could not be removed
// stays suspended with a note saying the import did not finish.
//
// # The cPanel full backup
//
// A full backup is what WHM's pkgacct writes (cpmove-USER.tar.gz) or what
// cPanel's Backup page offers as "full account backup"
// (backup-M.D.YYYY_HH-MM-SS_USER.tar.gz). Both are a gzip-compressed tar
// with one top folder, cpmove-USER/ or backup-…_USER/, holding:
//
//	homedir/                 the account's home folder (/home/USER): public_html
//	                         and the other document roots, mail/, etc/, ssl/, tmp/…
//	                         (archives from old cPanel versions have homedir.tar instead)
//	userdata/main            YAML: main_domain, addon_domains (addon: subdomain),
//	                         parked_domains, sub_domains
//	userdata/DOMAIN          YAML per website: servername, serveralias (space
//	                         separated), documentroot, homedir, user, phpversion
//	                         (ea-php81 means PHP 8.1); also DOMAIN_SSL, *.cache, cache.json
//	mysql/DB.sql             a dump of each database, without CREATE DATABASE or USE
//	mysql/DB.create          its CREATE DATABASE statement (not used: Nefo creates databases)
//	mysql.sql                the database users: GRANT USAGE ON *.* TO 'u'@'h'
//	                         IDENTIFIED BY PASSWORD '*HASH'; GRANT ALL PRIVILEGES ON
//	                         `USER\_db`.* TO 'u'@'h'; (MySQL 8 servers may write CREATE
//	                         USER … IDENTIFIED WITH 'plugin' AS 'hash' instead)
//	apache_tls/DOMAIN        private key and certificate chain of each domain, in one PEM
//	sslkeys/, sslcerts/      older layout: DOMAIN.key, DOMAIN.crt, DOMAIN.cabundle
//	                         (homedir/ssl/keys and homedir/ssl/certs hold copies)
//	cron/USER                the account's crontab
//	dnszones/DOMAIN.db       DNS zones          (not imported: Nefo runs no DNS server)
//	va/, vf/, vad/, mm/…     mail aliases, filters, mailing lists (not imported:
//	                         Nefo has no mail server; mailboxes are in homedir/mail)
//	proftpdpasswd            FTP accounts       (not imported: websites use SFTP)
//	psql/                    PostgreSQL databases (not imported)
//	cp/USER, shadow, quota, version, homedir_paths, pds, sds, sds2, addons…
//
// Sources: cPanel's "Backup Tarball Contents"
// (https://docs.cpanel.net/knowledge-base/backup/backup-tarball-contents/),
// "The pkgacct Script" (https://docs.cpanel.net/whm/scripts/the-pkgacct-script/),
// HestiaCP's importer (bin/v-import-cpanel: userdata/main, mysql/*.create,
// mysql.sql's GRANT USAGE … IDENTIFIED BY PASSWORD lines, apache_tls/,
// cron/USER) and Virtualmin's migration-cpanel.pl (cpmove-USER/ and
// backup-…_USER/ top folders, homedir.tar in old archives, the GRANT forms).
//
// # aaPanel
//
// aaPanel's site backups are web_DOMAIN_YYYYMMDD_HHMMSS_RANDOM.tar.gz,
// made with "cd /www/wwwroot && tar zcvhf … DOMAIN", so they have one top
// folder named after the site's folder; database backups are
// db_NAME_YYYYMMDD_HHMMSS_mysql_data.sql.gz (older versions:
// db_NAME_YYYYMMDD_HHMMSS.sql.gz), a mysqldump without CREATE DATABASE.
// Source: aaPanel's class_v2/panel_backup_v2.py. Its sites carry a
// .user.ini with open_basedir=/www/wwwroot/DOMAIN/, which tells the old
// folder.
//
// # MySQL 8 dumps
//
// Dumps from MySQL 8 (aaPanel's default, many cPanel servers) name
// collations MariaDB does not know (utf8mb4_0900_ai_ci…), carry
// /*!80016 … */ comments that MariaDB runs because its version number is
// higher, and may set GTID_PURGED or SQL_LOG_BIN, which MariaDB refuses.
// See sqlfix.go: the dump is adapted on the way in, as a stream.
package siteimport
