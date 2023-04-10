#!/bin/bash
set -ex

oc delete validatingwebhookconfiguration vovs.kb.io --ignore-not-found
oc delete mutatingwebhookconfiguration movs.kb.io --ignore-not-found
