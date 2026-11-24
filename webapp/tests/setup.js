// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import '@testing-library/jest-dom';

// jsdom does not implement pseudo-element styles, which Compass queries.
const {getComputedStyle} = window;
window.getComputedStyle = (elt) => getComputedStyle(elt);
