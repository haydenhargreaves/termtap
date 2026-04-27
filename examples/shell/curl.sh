#!/usr/bin/env bash

curl "${TERM_TAP_CURL_URL:-https://ipinfo.io:443/json}"
