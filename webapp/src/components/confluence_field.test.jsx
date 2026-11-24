import React from 'react';
import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import ConfluenceField from './confluence_field';

const options = [
    {label: 'Page Create', value: 'page_created'},
    {label: 'Page Update', value: 'page_updated'},
];

const baseProps = {
    required: true,
    onChange: jest.fn(),
    addValidation: jest.fn(),
    removeValidation: jest.fn(),
};

describe('components/ConfluenceField', () => {
    beforeEach(() => {
        jest.clearAllMocks();
    });

    test('labels a text input with its visible label and passes typed text through', async () => {
        render(
            <ConfluenceField
                {...baseProps}
                label='Space key'
                fieldType='input'
                value=''
                testId='subscription-space-key-input'
            />,
        );

        const input = screen.getByRole('textbox', {name: 'Space key'});
        expect(input).toBe(screen.getByTestId('subscription-space-key-input'));

        await userEvent.type(input, 'T');
        expect(baseProps.onChange).toHaveBeenCalledWith(expect.objectContaining({target: input}));
    });

    test('returns the chosen option objects from a multi select', async () => {
        render(
            <ConfluenceField
                {...baseProps}
                label='Events'
                fieldType='dropDown'
                name='events'
                value={[options[0]]}
                options={options}
                isMulti={true}
                testId='subscription-events-select'
            />,
        );

        const combobox = screen.getByRole('combobox', {name: 'Events'});
        expect(combobox).toHaveAttribute('id', 'subscription-events-select');

        await userEvent.click(combobox);
        await userEvent.click(await screen.findByRole('option', {name: 'Page Update'}));
        expect(baseProps.onChange).toHaveBeenLastCalledWith(options);
    });

    test('returns the chosen option object from a single select', async () => {
        render(
            <ConfluenceField
                {...baseProps}
                label='Subscribe to'
                fieldType='dropDown'
                name='type'
                value={options[0]}
                options={options}
                isMulti={false}
                testId='subscription-type-select'
            />,
        );

        await userEvent.click(screen.getByRole('combobox', {name: 'Subscribe to'}));
        await userEvent.click(await screen.findByRole('option', {name: 'Page Update'}));
        expect(baseProps.onChange).toHaveBeenCalledWith(options[1]);
    });

    test('flags an empty required field when validated', () => {
        let validate;
        render(
            <ConfluenceField
                {...baseProps}
                addValidation={(fn) => {
                    validate = fn;
                }}
                label='Name'
                fieldType='input'
                value=''
                testId='subscription-name-input'
            />,
        );

        let valid;
        React.act(() => {
            valid = validate();
        });

        expect(valid).toBe(false);
        expect(screen.getByRole('textbox', {name: 'Name'})).toHaveAccessibleDescription('This field is required.');
    });

    test.each([
        [false, 'Subscribe to'],
        [true, 'Events'],
    ])('marks an empty required select (isMulti=%s) as required and describes the error', (isMulti, label) => {
        let validate;
        render(
            <ConfluenceField
                {...baseProps}
                addValidation={(fn) => {
                    validate = fn;
                }}
                label={label}
                fieldType='dropDown'
                name='field'
                value={isMulti ? [] : null}
                options={options}
                isMulti={isMulti}
                testId='subscription-select'
            />,
        );

        React.act(() => {
            validate();
        });

        const combobox = screen.getByRole('combobox', {name: label});
        expect(combobox).toHaveAttribute('aria-required', 'true');
        expect(combobox).toHaveAccessibleDescription('This field is required.');
    });
});
