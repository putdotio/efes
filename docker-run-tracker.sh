#!/bin/bash -ex
efes ready mysql 2>/dev/null
efes ready rabbitmq 2>/dev/null

exec efes tracker
