package main

var testConfig *Config

func init() {
	c := defaultConfig
	c.Debug = true
	c.Database.DSN = "mogilefs:123@(efestest_mysql_1:3306)/mogilefs"
	c.AMQP.URL = "amqp://mogilefs:123@efestest_rabbitmq_1:5672/"
	testConfig = &c
}
