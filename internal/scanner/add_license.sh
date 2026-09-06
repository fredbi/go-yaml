#! /bin/bash
temp=$(mktemp -d)
for file in $(ls -1 *.go); do
  if ! head "${file}"|grep -q "SPDX" 1>/dev/null 2>/dev/null ; then
    {
      echo "// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers"
      echo "// SPDX-License-Identifier: Apache-2.0"
      echo ""
      cat "${file}"
    } > "${temp}"/"${file}"
   mv "${temp}"/"${file}" "${file}"
  fi
done


