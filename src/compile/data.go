package main

var InitScript = `
# ------------------------------------------------------------------------------------------------
# Copyright 2013 Jordon Bedwell.
# Apache License.
#
# Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
# except  in compliance with the License. You may obtain a copy of the License at:
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software distributed under the
# License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
# either express or implied. See the License for the specific language governing permissions
# and  limitations under the License.
# ------------------------------------------------------------------------------------------------

export APP_ROOT=$HOME
export LD_LIBRARY_PATH=$APP_ROOT/nginx/lib:$LD_LIBRARY_PATH

mv $APP_ROOT/nginx/conf/nginx.conf $APP_ROOT/nginx/conf/orig.conf
erb $APP_ROOT/nginx/conf/orig.conf > $APP_ROOT/nginx/conf/nginx.conf

if [[ ! -f $APP_ROOT/nginx/logs/access.log ]]; then
    mkfifo $APP_ROOT/nginx/logs/access.log
fi

if [[ ! -f $APP_ROOT/nginx/logs/error.log ]]; then
    mkfifo $APP_ROOT/nginx/logs/error.log
fi

cat < $APP_ROOT/nginx/logs/access.log &
(>&2 cat) < $APP_ROOT/nginx/logs/error.log &
`
