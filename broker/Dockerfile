# FROM node:14.16-alpine3.13 also works here for a smaller image
FROM node:16.5.0-alpine3.12

# set our node environment, either development or production
# defaults to production, compose overrides this to development on build and run
ENV NODE_ENV production

EXPOSE 9293 9296

# you'll likely want the latest npm, regardless of node version, for speed and fixes
# but pin this version for the best stability
RUN npm i npm@latest -g

# install dependencies first, in a different location for easier app bind mounting for local development
WORKDIR /opt/service-fabrik-broker/broker

COPY broker/package.json package.json
COPY broker/.yarn .yarn
COPY broker/yarn.lock yarn.lock
COPY broker/.yarnrc.yml .yarnrc.yml
COPY broker/applications/osb-broker applications/osb-broker
COPY broker/applications/quota-app applications/quota-app
COPY broker/core core
COPY broker/data-access-layer data-access-layer
RUN mkdir logs

RUN yarn set version berry
RUN yarn cache clean
RUN yarn workspaces focus @sf/osb-broker --production
RUN yarn workspaces focus @sf/quota-app --production
# copy in our source code last, as it changes the most

ENV CONF_DIR /opt/service-fabrik-broker/broker/config
ENV SETTINGS_PATH=$CONF_DIR/settings.yml

# the official node image provides an unprivileged user as a security best practice
# https://github.com/nodejs/docker-node/blob/master/docs/BestPractices.md#non-root-user
# USER node
