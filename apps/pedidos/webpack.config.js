const { NxAppWebpackPlugin } = require('@nx/webpack/app-plugin');
const { join } = require('path');
const { dependencies = {} } = require('./package.json');

// As libs das `dependencies` deste serviço ficam fora do bundle e carregam do
// node_modules em runtime. O Nx sozinho só faz isso para libs do node_modules
// da raiz, e as libs deste serviço moram em apps/<serviço>/node_modules.
const ownDependencies = Object.keys(dependencies);
const isOwnDependency = (request) =>
  ownDependencies.some((dep) => request === dep || request.startsWith(`${dep}/`));

module.exports = {
  output: {
    path: join(__dirname, 'dist'),
    clean: true,
    ...(process.env.NODE_ENV !== 'production' && {
      devtoolModuleFilenameTemplate: '[absolute-resource-path]',
    }),
  },
  externals: [
    ({ request }, callback) =>
      isOwnDependency(request) ? callback(null, `commonjs ${request}`) : callback(),
  ],
  plugins: [
    new NxAppWebpackPlugin({
      target: 'node',
      compiler: 'tsc',
      main: './src/main.ts',
      // Second bundle: dist/migrate.js (one-shot migrations container).
      additionalEntryPoints: [{ entryName: 'migrate', entryPath: './src/migrate.ts' }],
      tsConfig: './tsconfig.app.json',
      // drizzle/ (the SQL migrations) is copied to dist/drizzle for migrate.js.
      assets: ['./src/assets', { input: './drizzle', glob: '**/*', output: 'drizzle' }],
      optimization: false,
      outputHashing: 'none',
      generatePackageJson: false,
      // Junta os externals acima com os que o Nx calcula.
      mergeExternals: true,
      sourceMap: true,
    })
  ],
};
