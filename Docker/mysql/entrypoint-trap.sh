#!/bin/bash
# https://github.com/docker-library/mysql/issues/47#issuecomment-147397851
# A wrapper around /entrypoint.sh to trap the SIGINT signal (Ctrl+C) and forwards it to the mysql daemon
# In other words : traps SIGINT and SIGTERM signals and forwards them to the child process as SIGTERM signals

asyncRun() {
    "$@" &
    pid="$!"
    trap "echo 'Stopping PID $pid'; kill -SIGTERM $pid" SIGINT SIGTERM

    # A signal emitted while waiting will make the wait command return code > 128
    # Let's wrap it in a loop that doesn't end before the process is indeed stopped
    while kill -0 $pid > /dev/null 2>&1; do
        wait
    done
}
# call base entrypoint.sh that is already included in the image
asyncRun /entrypoint.sh $@