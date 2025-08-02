#!/usr/bin/env python3
"""
Test file for LeoPoacherBot timer functionality
Tests the /start_timer command with specific usernames
"""

import asyncio
import logging
from unittest.mock import Mock, AsyncMock, patch
from aiogram.enums import ChatMemberStatus
from aiogram.types import Message, User, Chat, ChatMemberUpdated, ChatMember

# Import the bot functions
import sys
import os
sys.path.append(os.path.dirname(os.path.abspath(__file__)))

from leo_bot import find_user_by_username, handle_message


class TestTimerFunctionality:
    """Test class for timer functionality"""
    
    def setup_method(self):
        """Setup for each test method"""
        self.bot_mock = Mock()
        self.bot_mock.get_chat_member = AsyncMock()
        self.bot_mock.resolve_peer = AsyncMock()
        self.bot_mock.send_message = AsyncMock()
        
        # Mock the bot instance
        import leo_bot
        leo_bot.bot = self.bot_mock
        
        # Setup logging
        logging.basicConfig(level=logging.INFO)
    
    async def test_find_user_by_username_success(self):
        """Test finding user by username successfully"""
        # Mock chat member
        chat_member = Mock()
        chat_member.status = ChatMemberStatus.MEMBER
        chat_member.user.username = "testuser"
        chat_member.user.first_name = "Test User"
        
        # Mock resolved peer
        resolved_peer = Mock()
        resolved_peer.user_id = 12345
        
        # Setup mocks
        self.bot_mock.resolve_peer.return_value = resolved_peer
        self.bot_mock.get_chat_member.return_value = chat_member
        
        # Test the function
        result = await find_user_by_username(chat_id=1, username="@testuser")
        
        # Verify the result
        assert result is not None
        assert result['user_id'] == 12345
        assert result['username'] == "testuser"
        assert result['status'] == ChatMemberStatus.MEMBER.value
        
        # Verify mocks were called correctly
        self.bot_mock.resolve_peer.assert_called_once_with("testuser")
        self.bot_mock.get_chat_member.assert_called_once_with(1, 12345)
    
    async def test_find_user_by_username_not_in_chat(self):
        """Test finding user by username when user is not in chat"""
        # Mock chat member with left status
        chat_member = Mock()
        chat_member.status = ChatMemberStatus.LEFT
        
        # Mock resolved peer
        resolved_peer = Mock()
        resolved_peer.user_id = 12345
        
        # Setup mocks
        self.bot_mock.resolve_peer.return_value = resolved_peer
        self.bot_mock.get_chat_member.return_value = chat_member
        
        # Test the function
        result = await find_user_by_username(chat_id=1, username="@testuser")
        
        # Verify the result
        assert result is None
        
        # Verify mocks were called correctly
        self.bot_mock.resolve_peer.assert_called_once_with("testuser")
        self.bot_mock.get_chat_member.assert_called_once_with(1, 12345)
    
    async def test_find_user_by_username_resolve_failed(self):
        """Test finding user by username when resolve fails"""
        # Setup mocks to fail
        self.bot_mock.resolve_peer.return_value = None
        
        # Test the function
        result = await find_user_by_username(chat_id=1, username="@testuser")
        
        # Verify the result
        assert result is None
        
        # Verify mocks were called correctly
        self.bot_mock.resolve_peer.assert_called_once_with("testuser")
        self.bot_mock.get_chat_member.assert_not_called()
    
    async def test_start_timer_command_with_usernames(self):
        """Test /start_timer command with specific usernames"""
        # Create mock message
        message = Mock()
        message.text = "/start_timer @user1 @user2"
        message.chat.id = 1
        message.from_user.id = 999  # Admin user
        
        # Mock chat member for admin
        admin_member = Mock()
        admin_member.status = ChatMemberStatus.ADMINISTRATOR
        
        # Mock chat members for target users
        user1_member = Mock()
        user1_member.status = ChatMemberStatus.MEMBER
        user1_member.user.username = "user1"
        user1_member.user.first_name = "User One"
        
        user2_member = Mock()
        user2_member.status = ChatMemberStatus.MEMBER
        user2_member.user.username = "user2"
        user2_member.user.first_name = "User Two"
        
        # Mock resolved peers
        resolved_peer1 = Mock()
        resolved_peer1.user_id = 111
        resolved_peer2 = Mock()
        resolved_peer2.user_id = 222
        
        # Setup mocks
        self.bot_mock.get_chat_member.side_effect = [
            admin_member,  # For admin check
            user1_member,  # For user1
            user2_member   # For user2
        ]
        self.bot_mock.resolve_peer.side_effect = [
            resolved_peer1,  # For user1
            resolved_peer2   # For user2
        ]
        
        # Mock message reply
        message.reply = AsyncMock()
        
        # Test the function
        await handle_message(message)
        
        # Verify admin check was called
        self.bot_mock.get_chat_member.assert_called_with(1, 999)
        
        # Verify resolve_peer was called for both users
        self.bot_mock.resolve_peer.assert_any_call("user1")
        self.bot_mock.resolve_peer.assert_any_call("user2")
        
        # Verify message reply was called
        message.reply.assert_called_once()
        
        # Check that the reply contains success message
        reply_call = message.reply.call_args[0][0]
        assert "Таймеры запущены для 2 пользователей" in reply_call
        assert "@user1" in reply_call
        assert "@user2" in reply_call


async def run_tests():
    """Run all tests"""
    test_instance = TestTimerFunctionality()
    
    print("🧪 Running timer functionality tests...")
    
    # Test finding user by username
    test_instance.setup_method()
    await test_instance.test_find_user_by_username_success()
    print("✅ test_find_user_by_username_success passed")
    
    test_instance.setup_method()
    await test_instance.test_find_user_by_username_not_in_chat()
    print("✅ test_find_user_by_username_not_in_chat passed")
    
    test_instance.setup_method()
    await test_instance.test_find_user_by_username_resolve_failed()
    print("✅ test_find_user_by_username_resolve_failed passed")
    
    test_instance.setup_method()
    await test_instance.test_start_timer_command_with_usernames()
    print("✅ test_start_timer_command_with_usernames passed")
    
    print("🎉 All tests passed!")


if __name__ == "__main__":
    asyncio.run(run_tests()) 