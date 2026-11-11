import React from 'react';
import {render, screen} from '@testing-library/react';

import ConfluenceField from './confluence_field';

const baseProps = {
    required: true,
    onChange: jest.fn(),
    addValidation: jest.fn(),
    removeValidation: jest.fn(),
};

describe('components/ConfluenceField', () => {
    test('labels a text input with its visible label', () => {
        render(
            <ConfluenceField
                {...baseProps}
                label='Space Key'
                fieldType='input'
                value=''
                testId='subscription-space-key-input'
            />,
        );

        expect(screen.getByRole('textbox', {name: 'Space Key'})).toBe(screen.getByTestId('subscription-space-key-input'));
    });

    test('labels a dropdown input with its visible label', () => {
        render(
            <ConfluenceField
                {...baseProps}
                label='Events'
                fieldType='dropDown'
                name='events'
                value={[]}
                options={[{label: 'Page Created', value: 'page_created'}]}
                isMulti={true}
                testId='subscription-events-select'
            />,
        );

        expect(screen.getByRole('combobox', {name: 'Events'})).toHaveAttribute('id', 'subscription-events-select');
    });
});
