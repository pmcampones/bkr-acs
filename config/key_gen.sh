#!/usr/bin/bash

if [ $# -ne 1 ]; then
    echo "Usage: $0 <num_keys>"
    exit 1
fi

NUM_KEYS=$1

for I in $(seq 1 $NUM_KEYS);
do
    openssl ecparam -name prime256v1 -genkey -noout -out sk$I.pem
    openssl ec -in sk$I.pem -pubout -out pk$I.pem
    openssl req -new -x509 -key sk$I.pem -out c$I.pem -days 99999 -batch
done
