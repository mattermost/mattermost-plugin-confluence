const config = {
    presets: [
        ['@babel/preset-env', {
            targets: {
                chrome: 66,
                firefox: 60,
                edge: 42,
                safari: 12,
            },
            modules: false,
            corejs: 3,
            debug: false,
            useBuiltIns: 'usage',
            shippedProposals: true,
        }],
        ['@babel/preset-react', {
            useBuiltIns: true,
        }],
        ['@babel/preset-typescript', {
            allExtensions: true,
            isTSX: true,
        }],
    ],
    // Don't specify plugins directly, let preset-env handle them
};

// Jest needs module transformation
config.env = {
    test: {
        presets: config.presets,
    },
};
config.env.test.presets[0][1].modules = 'auto';

module.exports = config;
