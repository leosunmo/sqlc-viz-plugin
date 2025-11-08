#!/bin/bash


docker run --name sqlc-viz-db -d --rm -e POSTGRES_PASSWORD=password -p 5432:5432 postgres:16.4-alpine
