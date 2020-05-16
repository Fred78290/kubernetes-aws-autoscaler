#!/bin/bash
set -e

go mod vendor

export Test_AuthMethodKey=NO
export Test_Sudo=NO
export Test_getVM=YES
export Test_listVM=YES
export Test_createVM=YES
export Test_statusVM=YES
export Test_powerOnVM=YES
export Test_powerOffVM=YES
export Test_shutdownGuest=YES
export Test_deleteVM=YES

echo "Run test"
#go test --test.short -race ./aws
