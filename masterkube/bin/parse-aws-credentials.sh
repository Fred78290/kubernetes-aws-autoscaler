#!/bin/bash

# Read and parse single section in INI file 


# Get/Set single INI section
GetINISection() {
  local filename="$1"
  local section="$2"
  local name=$(echo -n "$section" | tr '-' '_')

  array_name="configuration_${name}"

  declare -g -A ${array_name}

  eval $(awk -v configuration_array="${array_name}" \
             -v members="$section" \
             -F= '{ 
                    if ($1 ~ /^\[/) 
                      section=tolower(gensub(/\[(.+)\]/,"\\1",1,$1)) 
                    else if ($1 !~ /^$/ && $1 !~ /^;/) {
                      gsub(/^[ \t]+|[ \t]+$/, "", $1); 
                      gsub(/[\[\]]/, "", $1);
                      gsub(/^[ \t]+|[ \t]+$/, "", $2);
                      if (section == members) {
                        if (configuration[section][$1] == "")  
                          configuration[section][$1]=$2
                        else
                          configuration[section][$1]=configuration[section][$1]" "$2}
                      }
                    } 
                    END {
                        for (key in configuration[members])  
                          print configuration_array"[\""key"\"]=\""configuration[members][key]"\";"
                    }' ${filename}
        )
}

get_access_key() {
    local filename=~/.aws/credentials

    if [ -f $filename ] && [ -n "$1" ]; then
        local section="$1"
        local name=$(echo -n "$section" | tr '-' '_')
        
        GetINISection $filename "$section"

        for key in $(eval echo $\{'!'configuration_${name}[@]\}); do
            eval "${key}=$(eval echo $\{configuration_${name}[$key]\})"
        done
    else
        echo "missing INI file and/or INI section"

        exit -1
    fi
}
