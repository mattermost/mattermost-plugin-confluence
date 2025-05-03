// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import Enzyme from 'enzyme';
import Adapter from 'enzyme-adapter-react-16';
import '@testing-library/jest-dom';
import 'jest-expect-message';
import { TextEncoder, TextDecoder } from 'text-encoding';

// Add TextEncoder/TextDecoder polyfills
global.TextEncoder = TextEncoder;
global.TextDecoder = TextDecoder;

// Required for React 17 compatibility with enzyme
// See https://github.com/enzymejs/enzyme/issues/2429
jest.mock('react', () => ({
    ...jest.requireActual('react'),
    useMemo: (fn) => fn(),
}));

// Configure Enzyme for React 17 compatibility
// enzyme-adapter-react-16 is compatible with React 17 for most use cases
Enzyme.configure({adapter: new Adapter()});