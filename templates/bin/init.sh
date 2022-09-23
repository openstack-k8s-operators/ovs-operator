#!/bin//bash
#
# Copyright 2022 Red Hat Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License"); you may
# not use this file except in compliance with the License. You may obtain
# a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
# WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
# License for the specific language governing permissions and limitations
# under the License.
set -ex

# Configs are obtained from ENV variables.
export OvnBridge=${OvnBridge:-"br-int"}
export OvnRemote=${OvnRemote:-"tcp:127.0.0.1:6642"}
export OvnEncapType=${OvnEncapType:-"geneve"}
export OvnEncapIP=${OvnEncapIP:-"127.0.0.1"}

# configure external-ids in OVS
ovs-vsctl set open . external-ids:ovn-remote=${OvnRemote}
ovs-vsctl set open . external-ids:ovn-encap-type=${OvnEncapType}
ovs-vsctl set open . external-ids:ovn-encap-ip=${OvnEncapIP}