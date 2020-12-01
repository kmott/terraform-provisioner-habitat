#!/bin/bash
#
# A helper script that invokes the inspec against the specified target machine
#
__TARGET="${1}"

if [[ -n "${__TARGET}" ]]; then
  case "${__TARGET}" in
    linux)
      pushd ../terraform >/dev/null
      __PAYLOAD="$( terraform output -json | jq -r "$( printf '."%s" | .value' "${__TARGET}" )" )"
      __NAME="$( echo "${__PAYLOAD}" | jq -r '.name' )"
      __ADDRESS="$( echo "${__PAYLOAD}" | jq -r '.address' )"
      popd >/dev/null

      echo "Invoking Inspec against '${__NAME}' using profile 'default' ..."
      chef exec inspec exec linux -t ssh://root@${__ADDRESS} -i ~/.ssh/klm-id_rsa.pem
      ;;

    supervisor-ring)
      pushd ../terraform >/dev/null
      __PAYLOAD="$( terraform output -json | jq -r "$( printf '."%s" | .value' "${__TARGET}" )" )"
      popd >/dev/null

      for INSPEC_TARGET in $( echo "${__PAYLOAD}" | jq -r '.[] | @base64' ) ; do
        function _jq() { echo "${1}" | jq -Rr '@base64d' | jq -r "${2}"; }

        __NAME="$( _jq "${INSPEC_TARGET}" | jq -r '.name' )"
        __ADDRESS="$( _jq "${INSPEC_TARGET}" | jq -r '.address' )"

        echo "Invoking Inspec against '${__NAME}' using profile 'default' ..."
        chef exec inspec exec linux -t ssh://root@${__ADDRESS} -i ~/.ssh/klm-id_rsa.pem
      done
      ;;

    windows)
      pushd ../terraform >/dev/null
      __PAYLOAD="$( terraform output -json | jq -r "$( printf '."%s" | .value' "${__TARGET}" )" )"
      __NAME="$( echo "${__PAYLOAD}" | jq -r '.name' )"
      __ADDRESS="$( echo "${__PAYLOAD}" | jq -r '.address' )"
      popd >/dev/null

      echo "Invoking Inspec against '${__NAME}' using profile 'windows' ..."
      chef exec inspec exec windows -t winrm://Administrator@${__ADDRESS} --password 'packer' --ssl --self-signed
      ;;

    *)
      echo "Unknown target '${__TARGET}', try one of: linux|supervisor-ring"
      exit 1
      ;;
  esac
else
  echo "Usage: $( basename "${0}" ) <linux|supervisor-ring"
  exit 1
fi