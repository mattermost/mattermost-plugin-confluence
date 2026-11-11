/*eslint max-nested-callbacks: ["error", 4]*/

import React from 'react';
import {render, screen, waitFor, act} from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import Constants from '../../constants';

import SubscriptionModal from './subscription_modal';

const allEvents = Constants.CONFLUENCE_EVENTS.map((event) => event.value);

describe('components/SubscriptionModal', () => {
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

    const renderModal = async (props = {}) => {
        let result;
        await act(async () => {
            result = render(
                <SubscriptionModal
                    {...baseProps}
                    {...props}
                />,
            );
        });
        return result;
    };

    const nameInput = () => screen.getByRole('textbox', {name: 'Name'});
    const urlInput = () => screen.getByRole('textbox', {name: 'Confluence Base URL'});
    const save = () => userEvent.click(screen.getByRole('button', {name: 'Save Subscription'}));

    const chooseSubscriptionType = async (label) => {
        await userEvent.click(screen.getByRole('combobox', {name: 'Subscribe To'}));
        await userEvent.click(await screen.findByRole('option', {name: label}));
    };

    beforeEach(() => {
        jest.clearAllMocks();
    });

    test('renders the subscription fields and actions', async () => {
        await renderModal({visibility: true});

        expect(screen.getByRole('dialog', {name: 'Edit Your Confluence Subscription'})).toBeInTheDocument();
        expect(nameInput()).toBeInTheDocument();
        expect(urlInput()).toBeInTheDocument();
        expect(screen.getByRole('combobox', {name: 'Subscribe To'})).toHaveTextContent('Space');
        expect(screen.getByRole('textbox', {name: 'Space Key'})).toBeInTheDocument();
        expect(screen.getByRole('combobox', {name: 'Events'})).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Cancel'})).toBeInTheDocument();
    });

    test('saves a new space subscription with trimmed values', async () => {
        await renderModal({visibility: true});

        await userEvent.type(nameInput(), '   Abc   ');
        await userEvent.type(urlInput(), 'https://teST.com');
        await userEvent.type(screen.getByRole('textbox', {name: 'Space Key'}), 'test       ');
        await save();

        await waitFor(() => {
            expect(baseProps.saveChannelSubscription).toHaveBeenCalledWith({
                alias: 'Abc',
                baseURL: 'https://test.com',
                spaceKey: 'test',
                events: allEvents,
                channelID: 'abcabcabcabcabc',
                pageID: '',
                subscriptionType: 'space_subscription',
            });
        });
        expect(baseProps.editChannelSubscription).not.toHaveBeenCalled();
        expect(baseProps.close).toHaveBeenCalled();
    });

    test('saves a new page subscription after switching the type', async () => {
        await renderModal({visibility: true});

        await userEvent.type(nameInput(), 'Abc');
        await userEvent.type(urlInput(), 'https://test.com');
        await chooseSubscriptionType('Page');

        expect(screen.queryByRole('textbox', {name: 'Space Key'})).not.toBeInTheDocument();
        await userEvent.type(screen.getByRole('textbox', {name: 'Page ID'}), '1234');
        await save();

        await waitFor(() => {
            expect(baseProps.saveChannelSubscription).toHaveBeenCalledWith({
                alias: 'Abc',
                baseURL: 'https://test.com',
                spaceKey: '',
                events: allEvents,
                channelID: 'abcabcabcabcabc',
                pageID: '1234',
                subscriptionType: 'page_subscription',
            });
        });
    });

    test('saves only the events left after removing one', async () => {
        await renderModal({visibility: true});

        await userEvent.type(nameInput(), 'Abc');
        await userEvent.type(urlInput(), 'https://test.com');
        await userEvent.type(screen.getByRole('textbox', {name: 'Space Key'}), 'test');
        act(() => screen.getByRole('button', {name: 'Page Update. Press Delete to remove.'}).focus());
        await userEvent.keyboard('{Delete}');
        await save();

        await waitFor(() => {
            expect(baseProps.saveChannelSubscription).toHaveBeenCalledWith(expect.objectContaining({
                events: allEvents.filter((event) => event !== 'page_updated'),
            }));
        });
    });

    test('loads an existing space subscription and saves the edit', async () => {
        const subscription = {
            alias: 'Abc',
            baseURL: 'https://test.com',
            spaceKey: 'test',
            events: ['comment_created', 'page_created'],
            pageID: '',
        };
        await renderModal({subscription});

        expect(nameInput()).toHaveValue('Abc');
        expect(screen.getByRole('textbox', {name: 'Space Key'})).toHaveValue('test');
        expect(screen.getByRole('button', {name: 'Comment Create. Press Delete to remove.'})).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Page Update. Press Delete to remove.'})).not.toBeInTheDocument();

        await userEvent.clear(nameInput());
        await userEvent.type(nameInput(), 'Xyz');
        await save();

        await waitFor(() => {
            expect(baseProps.editChannelSubscription).toHaveBeenCalledWith({
                oldAlias: 'Abc',
                alias: 'Xyz',
                baseURL: 'https://test.com',
                spaceKey: 'test',
                events: ['comment_created', 'page_created'],
                channelID: 'abcabcabcabcabc',
                pageID: '',
                subscriptionType: 'space_subscription',
            });
        });
        expect(baseProps.saveChannelSubscription).not.toHaveBeenCalled();
    });

    test('loads an existing page subscription with the page type selected', async () => {
        const subscription = {
            alias: 'Abc',
            baseURL: 'https://test.com',
            spaceKey: '',
            events: allEvents,
            pageID: '1234',
        };
        await renderModal({subscription});

        expect(screen.getByRole('combobox', {name: 'Subscribe To'})).toHaveTextContent('Page');
        expect(screen.getByRole('textbox', {name: 'Page ID'})).toHaveValue('1234');

        await save();

        await waitFor(() => {
            expect(baseProps.editChannelSubscription).toHaveBeenCalledWith(expect.objectContaining({
                oldAlias: 'Abc',
                pageID: '1234',
                subscriptionType: 'page_subscription',
            }));
        });
    });

    test('blocks saving and describes each empty required field', async () => {
        await renderModal({visibility: true});

        await save();

        expect(await screen.findAllByText('This field is required.')).toHaveLength(3);
        expect(nameInput()).toHaveAttribute('aria-invalid', 'true');
        expect(nameInput()).toHaveAccessibleDescription('This field is required.');
        expect(baseProps.saveChannelSubscription).not.toHaveBeenCalled();
    });

    test('clears the required error once the field is filled', async () => {
        await renderModal({visibility: true});

        await save();
        await userEvent.type(nameInput(), 'Abc');

        expect(nameInput()).not.toHaveAttribute('aria-invalid');
        expect(screen.getAllByText('This field is required.')).toHaveLength(2);
    });

    test('shows the server error in an alert and keeps the modal open', async () => {
        const saveChannelSubscription = jest.fn().mockResolvedValue({error: {response: {text: 'Subscription already exists'}}});
        await renderModal({visibility: true, saveChannelSubscription});

        await userEvent.type(nameInput(), 'Abc');
        await userEvent.type(urlInput(), 'https://test.com');
        await userEvent.type(screen.getByRole('textbox', {name: 'Space Key'}), 'test');
        await save();

        expect(await screen.findByRole('alert')).toHaveTextContent('Subscription already exists');
        expect(baseProps.close).not.toHaveBeenCalled();
    });

    test('cancel closes the modal without saving', async () => {
        await renderModal({visibility: true});

        await userEvent.click(screen.getByRole('button', {name: 'Cancel'}));

        expect(baseProps.close).toHaveBeenCalled();
        expect(baseProps.saveChannelSubscription).not.toHaveBeenCalled();
    });

    test('escape closes the subscription type menu before the modal', async () => {
        await renderModal({visibility: true});

        await userEvent.click(screen.getByRole('combobox', {name: 'Subscribe To'}));
        expect(await screen.findByRole('listbox')).toBeInTheDocument();
        await userEvent.keyboard('{Escape}');

        await waitFor(() => {
            expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
        });
        expect(baseProps.close).not.toHaveBeenCalled();
    });

    test('loads plugin config on mount', async () => {
        await renderModal({visibility: true});

        expect(baseProps.getPluginConfig).toHaveBeenCalled();
    });
});
