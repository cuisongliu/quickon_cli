#!/usr/bin/env bash

helm repo add q-stable https://hub.qucheng.com/chartrepo/stable
helm repo update
helm upgrade -i openebs q-stable/longhorn -n quickon-storage --create-namespace
