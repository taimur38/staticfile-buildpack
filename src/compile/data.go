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

var NginxConfTemplate = `
worker_processes 1;
daemon off;

error_log <%= ENV["APP_ROOT"] %>/nginx/logs/error.log;
events { worker_connections 1024; }

http {
  charset utf-8;
  log_format cloudfoundry '$http_x_forwarded_for - $http_referer - [$time_local] "$request" $status $body_bytes_sent';
  access_log <%= ENV["APP_ROOT"] %>/nginx/logs/access.log cloudfoundry;
  default_type application/octet-stream;
  include mime.types;
  sendfile on;

  gzip on;
  gzip_disable "msie6";
  gzip_comp_level 6;
  gzip_min_length 1100;
  gzip_buffers 16 8k;
  gzip_proxied any;
  gunzip on;
  gzip_static always;
  gzip_types text/plain text/css text/js text/xml text/javascript application/javascript application/x-javascript application/json application/xml application/xml+rss;
  gzip_vary on;

  tcp_nopush on;
  keepalive_timeout 30;
  port_in_redirect off; # Ensure that redirects don't include the internal container PORT - <%= ENV["PORT"] %>
  server_tokens off;

  server {
    listen <%= ENV["PORT"] %>;
    server_name localhost;

    location / {
      root <%= ENV["APP_ROOT"] %>/public;
      <% if File.exists?(File.join(ENV["APP_ROOT"], "nginx/conf/.enable_pushstate")) %>
      if (!-e $request_filename) {
        rewrite ^(.*)$ / break;
      }
      <% end %>
      index index.html index.htm Default.htm;
      <% if File.exists?(File.join(ENV["APP_ROOT"], "nginx/conf/.enable_directory_index")) %>
        autoindex on;
      <% end %>
      <% if File.exists?(auth_file = File.join(ENV["APP_ROOT"], "nginx/conf/.htpasswd")) %>
        auth_basic "Restricted";                                #For Basic Auth
        auth_basic_user_file <%= auth_file %>;  #For Basic Auth
      <% end %>
      <% if ENV["FORCE_HTTPS"] %>
        if ($http_x_forwarded_proto != "https") {
          return 301 https://$host$request_uri;
        }
      <% end %>
      <% if File.exists?(File.join(ENV["APP_ROOT"], "nginx/conf/.enable_ssi")) %>
        ssi on;
      <% end %>
      <% if File.exists?(File.join(ENV["APP_ROOT"], "nginx/conf/.enable_hsts")) %>
        add_header Strict-Transport-Security "max-age=31536000";
      <% end %>
      <% if File.exists?(File.join(ENV["APP_ROOT"], "nginx/conf/.enable_location_include")) %>
        include <%= File.read(File.join(ENV["APP_ROOT"], "nginx/conf/.enable_location_include")).chomp %>;
      <% end %>
    }

  <% unless File.exists?(File.join(ENV["APP_ROOT"], "nginx/conf/.enable_dotfiles")) %>
    location ~ /\. {
      deny all;
      return 404;
    }
  <% end %>
  }
}
`
