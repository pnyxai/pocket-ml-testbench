# Use an official Node.js runtime as the base image
FROM node:18.20-alpine

# Create application directory
WORKDIR /usr/src/app

# Install pnpm
RUN npm install -g pnpm

# Define environment variable
ENV NODE_ENV production

# Install application dependencies
# A wildcard is used to ensure package.json and pnpm-lock.yaml are copied where available
COPY apps/nodejs/test-api/package*.json pnpm-lock.yaml* ./

# Install only production dependencies
RUN pnpm install

# Copy application source code to the container
COPY apps/nodejs/test-api .

# Build the app
RUN pnpm run build

# Run the app when the container launches
CMD [ "pnpm", "start" ]

# Expose the listening port
EXPOSE 3001