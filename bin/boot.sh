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

echo $HOME
export APP_ROOT=$HOME
export LD_LIBRARY_PATH=$APP_ROOT/nginx/lib:$LD_LIBRARY_PATH

conf_file=$APP_ROOT/app/nginx/conf/nginx.conf
if [ -f $APP_ROOT/app/public/nginx.conf ]
then
  conf_file=$APP_ROOT/app/public/nginx.conf
fi

mv $conf_file $APP_ROOT/app/nginx/conf/orig.conf
erb $APP_ROOT/app/nginx/conf/orig.conf > $APP_ROOT/app/nginx/conf/nginx.conf

# ------------------------------------------------------------------------------------------------

mkfifo $APP_ROOT/app/nginx/logs/access.log
mkfifo $APP_ROOT/app/nginx/logs/error.log

cat < $APP_ROOT/app/nginx/logs/access.log &
(>&2 cat) < $APP_ROOT/app/nginx/logs/error.log &

exec $APP_ROOT/app/nginx/sbin/nginx -p $APP_ROOT/app/nginx -c $APP_ROOT/app/nginx/conf/nginx.conf

# ------------------------------------------------------------------------------------------------
