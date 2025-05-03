module.exports = {
  snapshotSerializers: [
    "<rootDir>/node_modules/enzyme-to-json/serializer"
  ],
  testPathIgnorePatterns: [
    "/node_modules/",
    "/non_npm_dependencies/"
  ],
  clearMocks: true,
  collectCoverageFrom: [
    "src/**/*.{js,jsx}"
  ],
  coverageReporters: [
    "lcov",
    "text-summary"
  ],
  moduleNameMapper: {
    "^.+\\.(jpg|jpeg|png|gif|eot|otf|webp|svg|ttf|woff|woff2|mp4|webm|wav|mp3|m4a|aac|oga)$": "identity-obj-proxy",
    "^.+\\.(css|less|scss)$": "identity-obj-proxy",
    "^.*i18n.*\\.(json)$": "<rootDir>/tests/i18n_mock.json",
    "^bundle-loader\\?lazy\\!(.*)$": "$1",
    "undici": "<rootDir>/tests/mock-undici.js",
    "react-select": "<rootDir>/tests/mock-react-select.js"
  },
  moduleDirectories: [
    "",
    "node_modules",
    "non_npm_dependencies"
  ],
  reporters: [
    "default",
    "jest-junit"
  ],
  transformIgnorePatterns: [
    "node_modules/(?!react-native|react-router|mattermost-webapp)"
  ],
  setupFiles: [
    "jest-canvas-mock"
  ],
  setupFilesAfterEnv: [
    "<rootDir>/tests/setup.js"
  ],
  testEnvironment: "jsdom",
  testEnvironmentOptions: {
    url: "http://localhost:8065"
  },
  transform: {
    "^.+\\.(js|jsx|ts|tsx)$": "babel-jest"
  }
};