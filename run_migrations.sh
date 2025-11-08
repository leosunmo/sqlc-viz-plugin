#!/bin/bash

tern migrate --conn-string "postgres://postgres:password@localhost:5432/postgres?sslmode=disable" -m testdata/migrations
