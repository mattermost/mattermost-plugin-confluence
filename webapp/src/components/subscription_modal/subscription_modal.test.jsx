/*eslint max-nested-callbacks: ["error", 4]*/

import React from 'react';
import {render, screen, fireEvent, waitFor, act} from '@testing-library/react';

import Constants from '../../constants';

import SubscriptionModal from './subscription_modal';

describe('components/ChannelSettingsModal', () => {
    const baseProps = {
        visibility: false,
        subscription: {},
        close: jest.fn(),
        saveChannelSubscription: jest.fn().mockResolvedValue({}),
        currentChannelID: 'abcabcabcabcabc',
        editChannelSubscription: jest.fn().mockResolvedValue({}),
        getPluginConfig: jest.fn().mockResolvedValue({
            data: {
                supportedEvents: Constants.CONFLUENCE_EVENTS,
            },
        }),
    };

    beforeEach(() => {
        jest.clearAllMocks();
    });

    test('subscription modal renders when visible', async () => {
        const props = {
            ...baseProps,
            visibility: true,
        };

        await act(async () => {
            render(<SubscriptionModal {...props}/>);
        });

        expect(screen.getByText('Edit Your Confluence Subscription')).toBeInTheDocument();
        expect(screen.getByRole('textbox', {name: 'Name'})).toBeInTheDocument();
        expect(screen.getByRole('textbox', {name: 'Confluence Base URL'})).toBeInTheDocument();
        expect(screen.getByRole('combobox', {name: 'Subscribe To'})).toBeInTheDocument();
        expect(screen.getByRole('combobox', {name: 'Events'})).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Save Subscription'})).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Cancel'})).toBeInTheDocument();
    });

    test('new space subscription', async () => {
        const props = {
            ...baseProps,
            visibility: true,
        };

        await act(async () => {
            render(<SubscriptionModal {...props}/>);
        });

        const nameInput = screen.getByTestId('subscription-name-input');
        const urlInput = screen.getByTestId('subscription-url-input');
        const spaceKeyInput = screen.getByTestId('subscription-space-key-input');

        fireEvent.change(nameInput, {target: {value: 'Abc'}});
        fireEvent.change(urlInput, {target: {value: 'https://test.com'}});
        fireEvent.change(spaceKeyInput, {target: {value: 'test'}});

        const saveButton = screen.getByText('Save Subscription');
        fireEvent.click(saveButton);

        await waitFor(() => {
            expect(props.saveChannelSubscription).toHaveBeenCalledWith({
                alias: 'Abc',
                baseURL: 'https://test.com',
                spaceKey: 'test',
                events: Constants.CONFLUENCE_EVENTS.map((event) => event.value),
                channelID: 'abcabcabcabcabc',
                pageID: '',
                subscriptionType: 'space_subscription',
            });
        });

        expect(props.editChannelSubscription).not.toHaveBeenCalled();
    });

    test('edit space subscription', async () => {
        const subscription = {
            alias: 'Abc',
            baseURL: 'https://test.com',
            spaceKey: 'test',
            events: Constants.CONFLUENCE_EVENTS.map((e) => e.value),
            pageID: '',
            subscriptionType: Constants.SUBSCRIPTION_TYPE[0].value,
        };

        let rerender;
        await act(async () => {
            const result = render(
                <SubscriptionModal
                    {...baseProps}
                    visibility={true}
                />,
            );
            rerender = result.rerender;
        });

        const nameInput = screen.getByTestId('subscription-name-input');
        const urlInput = screen.getByTestId('subscription-url-input');
        const spaceKeyInput = screen.getByTestId('subscription-space-key-input');

        fireEvent.change(nameInput, {target: {value: 'Abc'}});
        fireEvent.change(urlInput, {target: {value: 'https://test.com'}});
        fireEvent.change(spaceKeyInput, {target: {value: 'test'}});

        await act(async () => {
            rerender(
                <SubscriptionModal
                    {...baseProps}
                    subscription={subscription}
                />,
            );
        });

        fireEvent.change(nameInput, {target: {value: 'Xyz'}});

        const saveButton = screen.getByText('Save Subscription');
        fireEvent.click(saveButton);

        await waitFor(() => {
            expect(baseProps.editChannelSubscription).toHaveBeenCalledWith({
                oldAlias: 'Abc',
                alias: 'Xyz',
                baseURL: 'https://test.com',
                spaceKey: 'test',
                events: Constants.CONFLUENCE_EVENTS.map((event) => event.value),
                channelID: 'abcabcabcabcabc',
                pageID: '',
                subscriptionType: 'space_subscription',
            });
        });
        expect(baseProps.saveChannelSubscription).not.toHaveBeenCalled();
    });

    test('new page subscription', async () => {
        const props = {
            ...baseProps,
            visibility: true,
        };

        await act(async () => {
            render(<SubscriptionModal {...props}/>);
        });

        const nameInput = screen.getByTestId('subscription-name-input');
        const urlInput = screen.getByTestId('subscription-url-input');

        fireEvent.change(nameInput, {target: {value: 'Abc'}});
        fireEvent.change(urlInput, {target: {value: 'https://test.com'}});

        fireEvent.click(screen.getByRole('combobox', {name: 'Subscribe To'}));
        fireEvent.pointerUp(await screen.findByRole('option', {name: 'Page'}));

        const pageIDInput = await screen.findByTestId('subscription-page-id-input');
        fireEvent.change(pageIDInput, {target: {value: '1234'}});

        const saveButton = screen.getByText('Save Subscription');
        fireEvent.click(saveButton);

        await waitFor(() => {
            expect(props.saveChannelSubscription).toHaveBeenCalledWith({
                alias: 'Abc',
                baseURL: 'https://test.com',
                spaceKey: '',
                events: Constants.CONFLUENCE_EVENTS.map((event) => event.value),
                channelID: 'abcabcabcabcabc',
                pageID: '1234',
                subscriptionType: 'page_subscription',
            });
        });

        expect(props.editChannelSubscription).not.toHaveBeenCalled();
    });

    test('edit page subscription', async () => {
        const subscription = {
            alias: 'Abc',
            baseURL: 'https://test.com',
            spaceKey: '',
            events: Constants.CONFLUENCE_EVENTS.map((e) => e.value),
            pageID: '1234',
            subscriptionType: Constants.SUBSCRIPTION_TYPE[1].value,
        };

        let rerender;
        await act(async () => {
            const result = render(
                <SubscriptionModal
                    {...baseProps}
                    visibility={true}
                />,
            );
            rerender = result.rerender;
        });

        const nameInput = screen.getByTestId('subscription-name-input');
        const urlInput = screen.getByTestId('subscription-url-input');

        fireEvent.change(nameInput, {target: {value: 'Abc'}});
        fireEvent.change(urlInput, {target: {value: 'https://test.com'}});

        fireEvent.click(screen.getByRole('combobox', {name: 'Subscribe To'}));
        fireEvent.pointerUp(await screen.findByRole('option', {name: 'Page'}));

        const pageIDInput = await screen.findByTestId('subscription-page-id-input');
        fireEvent.change(pageIDInput, {target: {value: '1234'}});

        await act(async () => {
            rerender(
                <SubscriptionModal
                    {...baseProps}
                    subscription={subscription}
                />,
            );
        });

        fireEvent.change(nameInput, {target: {value: 'Xyz'}});

        const saveButton = screen.getByText('Save Subscription');
        fireEvent.click(saveButton);

        await waitFor(() => {
            expect(baseProps.editChannelSubscription).toHaveBeenCalledWith({
                oldAlias: 'Abc',
                alias: 'Xyz',
                baseURL: 'https://test.com',
                spaceKey: '',
                events: Constants.CONFLUENCE_EVENTS.map((event) => event.value),
                channelID: 'abcabcabcabcabc',
                pageID: '1234',
                subscriptionType: 'page_subscription',
            });
        });
        expect(baseProps.saveChannelSubscription).not.toHaveBeenCalled();
    });

    test('subscription data clean', async () => {
        const props = {
            ...baseProps,
            visibility: true,
        };

        await act(async () => {
            render(<SubscriptionModal {...props}/>);
        });

        const nameInput = screen.getByTestId('subscription-name-input');
        const urlInput = screen.getByTestId('subscription-url-input');
        const spaceKeyInput = screen.getByTestId('subscription-space-key-input');

        fireEvent.change(nameInput, {target: {value: '   Abc   '}});
        fireEvent.change(urlInput, {target: {value: 'https://teST.com'}});
        fireEvent.change(spaceKeyInput, {target: {value: 'test       '}});

        const saveButton = screen.getByText('Save Subscription');
        fireEvent.click(saveButton);

        await waitFor(() => {
            expect(props.saveChannelSubscription).toHaveBeenCalledWith({
                alias: 'Abc',
                baseURL: 'https://test.com',
                spaceKey: 'test',
                events: Constants.CONFLUENCE_EVENTS.map((event) => event.value),
                channelID: 'abcabcabcabcabc',
                pageID: '',
                subscriptionType: 'space_subscription',
            });
        });

        expect(props.editChannelSubscription).not.toHaveBeenCalled();
    });

    test('blocks saving and flags empty required fields', async () => {
        await act(async () => {
            render(
                <SubscriptionModal
                    {...baseProps}
                    visibility={true}
                />,
            );
        });

        fireEvent.click(screen.getByRole('button', {name: 'Save Subscription'}));

        expect(await screen.findAllByText('This field is required.')).toHaveLength(3);
        expect(screen.getByRole('textbox', {name: 'Name'})).toHaveAttribute('aria-invalid', 'true');
        expect(baseProps.saveChannelSubscription).not.toHaveBeenCalled();
    });

    test('cancel closes the modal', async () => {
        const props = {
            ...baseProps,
            visibility: true,
        };

        await act(async () => {
            render(<SubscriptionModal {...props}/>);
        });

        const cancelButton = screen.getByText('Cancel');
        fireEvent.click(cancelButton);

        expect(props.close).toHaveBeenCalled();
    });

    test('loads plugin config on mount', async () => {
        const props = {
            ...baseProps,
            visibility: true,
        };

        await act(async () => {
            render(<SubscriptionModal {...props}/>);
        });

        await waitFor(() => {
            expect(props.getPluginConfig).toHaveBeenCalled();
        });
    });
});
