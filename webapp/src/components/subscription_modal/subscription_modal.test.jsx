/*eslint max-nested-callbacks: ["error", 3]*/

import React from 'react';
import {render, screen, fireEvent, waitFor} from '@testing-library/react';

import Constants from '../../constants';

import SubscriptionModal from './subscription_modal';

describe('components/ChannelSettingsModal', () => {
    const mockTheme = {
        centerChannelColor: '#3d3c40',
        centerChannelBg: '#ffffff',
        buttonBg: '#166de0',
        buttonColor: '#ffffff',
        linkColor: '#2389d7',
    };

    const baseProps = {
        theme: mockTheme,
        visibility: false,
        subscription: {},
        close: jest.fn(),
        saveChannelSubscription: jest.fn().mockResolvedValue({}),
        currentChannelID: 'abcabcabcabcabc',
        editChannelSubscription: jest.fn().mockResolvedValue({}),
    };

    beforeEach(() => {
        jest.clearAllMocks();
    });

    test('subscription modal renders when visible', () => {
        const props = {
            ...baseProps,
            visibility: true,
        };
        render(<SubscriptionModal {...props}/>);

        expect(screen.getByText('Edit Your Confluence Subscription')).toBeInTheDocument();
        expect(screen.getByText('Name')).toBeInTheDocument();
        expect(screen.getByText('Confluence Base URL')).toBeInTheDocument();
        expect(screen.getByText('Subscribe To')).toBeInTheDocument();
        expect(screen.getByText('Events')).toBeInTheDocument();
        expect(screen.getByText('Save Subscription')).toBeInTheDocument();
        expect(screen.getByText('Cancel')).toBeInTheDocument();
    });

    test('new space subscription', async () => {
        const props = {
            ...baseProps,
            visibility: true,
        };
        render(<SubscriptionModal {...props}/>);

        const nameInput = screen.getByPlaceholderText('Enter a name for this subscription.');
        const urlInput = screen.getByPlaceholderText('Enter the Confluence Base URL.');
        const spaceKeyInput = screen.getByPlaceholderText('Enter the Confluence Space Key.');

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

        const {rerender} = render(<SubscriptionModal {...baseProps} visibility={true}/>);

        const nameInput = screen.getByPlaceholderText('Enter a name for this subscription.');
        const urlInput = screen.getByPlaceholderText('Enter the Confluence Base URL.');
        const spaceKeyInput = screen.getByPlaceholderText('Enter the Confluence Space Key.');

        fireEvent.change(nameInput, {target: {value: 'Abc'}});
        fireEvent.change(urlInput, {target: {value: 'https://test.com'}});
        fireEvent.change(spaceKeyInput, {target: {value: 'test'}});

        rerender(<SubscriptionModal {...baseProps} subscription={subscription}/>);

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
        render(<SubscriptionModal {...props}/>);

        const nameInput = screen.getByPlaceholderText('Enter a name for this subscription.');
        const urlInput = screen.getByPlaceholderText('Enter the Confluence Base URL.');

        fireEvent.change(nameInput, {target: {value: 'Abc'}});
        fireEvent.change(urlInput, {target: {value: 'https://test.com'}});

        const subscribeToDropdown = screen.getByText('Space');
        fireEvent.mouseDown(subscribeToDropdown);

        await waitFor(() => {
            const pageOption = screen.getByText('Page');
            fireEvent.click(pageOption);
        });

        await waitFor(() => {
            const pageIDInput = screen.getByPlaceholderText('Enter the page id.');
            fireEvent.change(pageIDInput, {target: {value: '1234'}});
        });

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

        const {rerender} = render(<SubscriptionModal {...baseProps} visibility={true}/>);

        const nameInput = screen.getByPlaceholderText('Enter a name for this subscription.');
        const urlInput = screen.getByPlaceholderText('Enter the Confluence Base URL.');

        fireEvent.change(nameInput, {target: {value: 'Abc'}});
        fireEvent.change(urlInput, {target: {value: 'https://test.com'}});

        const subscribeToDropdown = screen.getByText('Space');
        fireEvent.mouseDown(subscribeToDropdown);
        await waitFor(() => {
            const pageOption = screen.getByText('Page');
            fireEvent.click(pageOption);
        });

        await waitFor(() => {
            const pageIDInput = screen.getByPlaceholderText('Enter the page id.');
            fireEvent.change(pageIDInput, {target: {value: '1234'}});
        });

        rerender(<SubscriptionModal {...baseProps} subscription={subscription}/>);

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
        render(<SubscriptionModal {...props}/>);

        const nameInput = screen.getByPlaceholderText('Enter a name for this subscription.');
        const urlInput = screen.getByPlaceholderText('Enter the Confluence Base URL.');
        const spaceKeyInput = screen.getByPlaceholderText('Enter the Confluence Space Key.');

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

    test('cancel closes the modal', () => {
        const props = {
            ...baseProps,
            visibility: true,
        };
        render(<SubscriptionModal {...props}/>);

        const cancelButton = screen.getByText('Cancel');
        fireEvent.click(cancelButton);

        expect(props.close).toHaveBeenCalled();
    });
});
